import Foundation
import UniformTypeIdentifiers
import WebKit

/// Serves the immutable web engine and both original discs from the app bundle.
/// The game never needs a localhost server or a separately mounted disc image.
final class GameSchemeHandler: NSObject, WKURLSchemeHandler {
    private final class Load {
        let task: WKURLSchemeTask
        private let lock = NSLock()
        private var stopped = false

        init(_ task: WKURLSchemeTask) { self.task = task }
        var cancelled: Bool {
            lock.lock()
            defer { lock.unlock() }
            return stopped
        }
        func cancel() {
            lock.lock()
            stopped = true
            lock.unlock()
        }
    }

    private let roots: [String: URL]
    private let immutableGameFiles: Bool
    private let work = DispatchQueue(label: "com.titanic.restoration.resources", qos: .userInitiated, attributes: .concurrent)
    // WebKit starts and stops URL scheme tasks on the main thread.
    private var loads: [ObjectIdentifier: Load] = [:]

    init(webRoot: URL, gameRoot: URL) {
        roots = ["app": webRoot.resolvingSymlinksInPath(), "game": gameRoot.resolvingSymlinksInPath()]
        immutableGameFiles = gameRoot.resolvingSymlinksInPath().path.hasPrefix(Bundle.main.bundleURL.path + "/")
        super.init()
    }

    func webView(_ webView: WKWebView, start urlSchemeTask: WKURLSchemeTask) {
        let identity = ObjectIdentifier(urlSchemeTask as AnyObject)
        let load = Load(urlSchemeTask)
        loads[identity] = load
        work.async { [weak self] in
            guard let self else { return }
            do {
                try self.serve(load)
            } catch {
                self.deliver(load) { $0.didFailWithError(error) }
            }
            DispatchQueue.main.async { [weak self] in
                guard let self, self.loads[identity] === load else { return }
                self.loads.removeValue(forKey: identity)
            }
        }
    }

    func webView(_ webView: WKWebView, stop urlSchemeTask: WKURLSchemeTask) {
        let identity = ObjectIdentifier(urlSchemeTask as AnyObject)
        loads.removeValue(forKey: identity)?.cancel()
    }

    private func deliver(_ load: Load, _ action: @escaping (WKURLSchemeTask) -> Void) {
        DispatchQueue.main.sync {
            guard !load.cancelled else { return }
            action(load.task)
        }
    }

    private func serve(_ load: Load) throws {
        guard !load.cancelled, let url = load.task.request.url,
              let host = url.host?.lowercased(), var root = roots[host],
              url.scheme == "titanic", url.user == nil, url.password == nil,
              url.port == nil,
              let encodedPath = URLComponents(url: url, resolvingAgainstBaseURL: false)?.percentEncodedPath,
              let decodedPath = encodedPath.removingPercentEncoding,
              !decodedPath.contains("\0"), !decodedPath.contains("\\") else {
            throw URLError(.badURL)
        }
        let components = decodedPath.split(separator: "/", omittingEmptySubsequences: true)
        guard !components.contains(".."), !components.contains(".") else { throw URLError(.noPermissionsToReadFile) }
        let relativePath = components.isEmpty && host == "app" ? "index.html" : components.joined(separator: "/")
        if host == "app", ["gamefiles", "gamefiles.json", "templates"].contains(components.first.map(String.init) ?? "") {
            root = roots["game"]!
        }
        let file = root.appendingPathComponent(relativePath).resolvingSymlinksInPath()
        guard file.path.hasPrefix(root.path + "/") else { throw URLError(.noPermissionsToReadFile) }

        let method = load.task.request.httpMethod ?? "GET"
        if method == "OPTIONS" {
            sendHeaders(load, url: url, status: 204, fields: ["Content-Length": "0"])
            deliver(load) { $0.didFinish() }
            return
        }
        guard method == "GET" || method == "HEAD" else {
            sendHeaders(load, url: url, status: 405, fields: ["Allow": "GET, HEAD, OPTIONS", "Content-Length": "0"])
            deliver(load) { $0.didFinish() }
            return
        }

        guard let attributes = try? FileManager.default.attributesOfItem(atPath: file.path),
              attributes[.type] as? FileAttributeType == .typeRegular,
              let sizeNumber = attributes[.size] as? NSNumber else {
            sendHeaders(load, url: url, status: 404, fields: ["Content-Length": "0"])
            deliver(load) { $0.didFinish() }
            return
        }

        let size = sizeNumber.int64Value
        var lower: Int64 = 0
        var upper = max(0, size - 1)
        var status = 200
        if let range = load.task.request.value(forHTTPHeaderField: "Range") {
            guard let bounds = Self.byteRange(range, size: size) else {
                sendHeaders(load, url: url, status: 416, fields: ["Content-Range": "bytes */\(size)", "Content-Length": "0"])
                deliver(load) { $0.didFinish() }
                return
            }
            lower = bounds.0
            upper = bounds.1
            status = 206
        }
        let length = size == 0 ? 0 : upper - lower + 1
        var headers = [
            "Content-Type": Self.mimeType(file.pathExtension),
            "Content-Length": String(length),
            "Accept-Ranges": "bytes",
            // Imported media can be replaced between voyages without changing
            // its URL. Do not let WebKit reuse an earlier disc's cached bytes.
            "Cache-Control": root == roots["game"] && !immutableGameFiles ? "no-store" : "public, max-age=31536000, immutable"
        ]
        if status == 206 { headers["Content-Range"] = "bytes \(lower)-\(upper)/\(size)" }
        sendHeaders(load, url: url, status: status, fields: headers)
        guard method != "HEAD", length > 0 else {
            deliver(load) { $0.didFinish() }
            return
        }

        let handle = try FileHandle(forReadingFrom: file)
        defer { try? handle.close() }
        try handle.seek(toOffset: UInt64(lower))
        var remaining = length
        while remaining > 0 && !load.cancelled {
            guard let chunk = try handle.read(upToCount: Int(min(remaining, 1_048_576))), !chunk.isEmpty else {
                throw URLError(.cannotDecodeRawData)
            }
            remaining -= Int64(chunk.count)
            deliver(load) { $0.didReceive(chunk) }
        }
        if !load.cancelled { deliver(load) { $0.didFinish() } }
    }

    private func sendHeaders(_ load: Load, url: URL, status: Int, fields: [String: String]) {
        var headers = fields
        headers["Access-Control-Allow-Origin"] = "*"
        headers["Access-Control-Allow-Methods"] = "GET, HEAD, OPTIONS"
        headers["Access-Control-Allow-Headers"] = "Range, Content-Type"
        headers["Access-Control-Expose-Headers"] = "Content-Length, Content-Range, Accept-Ranges"
        if let response = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers) {
            deliver(load) { $0.didReceive(response) }
        }
    }

    private static func byteRange(_ value: String, size: Int64) -> (Int64, Int64)? {
        guard size > 0, value.hasPrefix("bytes="), !value.contains(",") else { return nil }
        let range = value.dropFirst(6).split(separator: "-", omittingEmptySubsequences: false)
        guard range.count == 2 else { return nil }
        if range[0].isEmpty {
            guard let suffix = Int64(range[1]), suffix > 0 else { return nil }
            return (max(0, size - suffix), size - 1)
        }
        guard let start = Int64(range[0]), start >= 0, start < size else { return nil }
        let end: Int64
        if range[1].isEmpty { end = size - 1 }
        else if let requestedEnd = Int64(range[1]), requestedEnd >= start { end = min(requestedEnd, size - 1) }
        else { return nil }
        return (start, end)
    }

    private static func mimeType(_ pathExtension: String) -> String {
        switch pathExtension.lowercased() {
        case "html": return "text/html; charset=utf-8"
        case "js", "mjs": return "application/javascript; charset=utf-8"
        case "css": return "text/css; charset=utf-8"
        case "json": return "application/json"
        case "wasm": return "application/wasm"
        case "svg": return "image/svg+xml"
        case "wav": return "audio/wav"
        case "mp4": return "video/mp4"
        default: return UTType(filenameExtension: pathExtension)?.preferredMIMEType ?? "application/octet-stream"
        }
    }
}

import Darwin
import Foundation

/// Imports user-owned, extracted English CD files without executing the Windows
/// installer. A complete new tree is published with one atomic directory move.
struct GameImporter {
    struct Progress {
        let completedBytes: Int64
        let totalBytes: Int64
        let message: String
        var fraction: Double { totalBytes > 0 ? Double(completedBytes) / Double(totalBytes) : 0 }
    }

    struct Result {
        let directory: URL
        let files: Int
        let bytes: Int64
    }

    final class Cancellation {
        private let lock = NSLock()
        private var stopped = false
        func cancel() { lock.lock(); stopped = true; lock.unlock() }
        var isCancelled: Bool { lock.lock(); defer { lock.unlock() }; return stopped }
        func check() throws { if isCancelled { throw ImportError.cancelled } }
    }

    enum ImportError: LocalizedError {
        case cancelled
        case missingDiscs
        case incompleteDisc(Int, String)
        case unsafeFile(String)
        case changedFile(String)
        case invalidManifest
        case publishFailed(Int32)

        var errorDescription: String? {
            switch self {
            case .cancelled: return "Import cancelled. Your existing game files are unchanged."
            case .missingDiscs:
                return "Both English game discs are needed. Choose the parent folder containing cd1 and cd2, or select both extracted disc folders together. Each disc folder should contain DATA and MOVIES."
            case .incompleteDisc(let number, let detail):
                return "Disc \(number) is incomplete: \(detail). Extract the entire disc, then choose its folder again."
            case .unsafeFile(let file): return "\(file) is a link or an unsupported file. Choose folders containing the original extracted disc files."
            case .changedFile(let file): return "\(file) changed while it was being copied. Finish extracting the discs before importing them."
            case .invalidManifest: return "The imported game files could not be verified. Your existing game files are unchanged."
            case .publishFailed(let code): return "The completed import could not be installed: \(String(cString: strerror(code))). Your existing game files are unchanged."
            }
        }
    }

    private struct SourceFile {
        let source: URL
        let relativePath: String
        let size: Int64
        let modified: Date?
    }

    private static let mediaExtensions: Set<String> = ["set", "trk", "stg", "sfx", "shp", "mov", "pup", "cst", "11k", "snd", "pic"]
    private static let excludedFolders: Set<String> = ["install", "support", "shots", "sneak"]
    private static let required: [Int: [String]] = [
        1: ["data/bootfile", "data/bedsit1.set", "data/main.stg", "data/ctl.stg"],
        2: ["data/a14.set", "data/deckbd.set", "data/cargo.set"]
    ]

    static func isInstalled(at directory: URL) -> Bool {
        guard let data = try? Data(contentsOf: directory.appendingPathComponent("gamefiles.json")),
              let manifest = try? JSONSerialization.jsonObject(with: data) as? [String: NSNumber],
              !manifest.isEmpty else { return false }
        let lowercased = Dictionary(manifest.map { ($0.key.lowercased(), $0) }, uniquingKeysWith: { first, _ in first })
        for disc in [1, 2] {
            for relative in GameFileIndex.requiredPaths[disc]! {
                let key = "gamefiles/en/titanic\(disc)/" + relative
                guard let entry = lowercased[key], entry.value.int64Value > 0,
                      let file = safeManifestFile(entry.key, in: directory),
                      let size = try? file.resourceValues(forKeys: [.fileSizeKey]).fileSize,
                      Int64(size) == entry.value.int64Value else { return false }
            }
        }
        return true
    }

    private static func safeManifestFile(_ path: String, in directory: URL) -> URL? {
        guard !path.hasPrefix("/"), !path.contains("\\"), !path.contains("\0"),
              !path.split(separator: "/").contains("..") else { return nil }
        let root = directory.resolvingSymlinksInPath()
        let file = root.appendingPathComponent(path).resolvingSymlinksInPath()
        return file.path.hasPrefix(root.path + "/") ? file : nil
    }

    func importDiscs(from selections: [URL], to destination: URL, cancellation: Cancellation,
                     progress: @escaping (Progress) -> Void) throws -> Result {
        try cancellation.check()
        progress(Progress(completedBytes: 0, totalBytes: 0, message: "Checking both discs…"))
        let discs = try discoverDiscs(selections, cancellation: cancellation)
        var files: [SourceFile] = []
        for disc in [1, 2] {
            let inventory = try inventory(disc: disc, root: discs[disc]!, cancellation: cancellation)
            try validate(disc: disc, files: inventory)
            files.append(contentsOf: inventory)
        }
        let total = files.reduce(Int64(0)) { $0 + $1.size }
        let manager = FileManager.default
        let parent = destination.deletingLastPathComponent()
        try manager.createDirectory(at: parent, withIntermediateDirectories: true)
        let staging = parent.appendingPathComponent(".Game-import-\(UUID().uuidString)", isDirectory: true)
        try manager.createDirectory(at: staging, withIntermediateDirectories: false)
        // After an atomic swap, staging contains the old tree. Remove it only
        // once the complete replacement has been published successfully.
        defer { try? manager.removeItem(at: staging) }
        var manifest: [String: Int64] = [:]
        var completed: Int64 = 0
        var lastProgress = Date.distantPast
        for file in files {
            try cancellation.check()
            let output = staging.appendingPathComponent(file.relativePath)
            try manager.createDirectory(at: output.deletingLastPathComponent(), withIntermediateDirectories: true)
            guard manager.createFile(atPath: output.path, contents: nil) else {
                throw CocoaError(.fileWriteUnknown, userInfo: [NSFilePathErrorKey: output.path])
            }
            let input = try FileHandle(forReadingFrom: file.source)
            let writer = try FileHandle(forWritingTo: output)
            do {
                var copied: Int64 = 0
                while let chunk = try input.read(upToCount: 1_048_576), !chunk.isEmpty {
                    try cancellation.check()
                    try writer.write(contentsOf: chunk)
                    copied += Int64(chunk.count)
                    completed += Int64(chunk.count)
                    if Date().timeIntervalSince(lastProgress) > 0.1 {
                        progress(Progress(completedBytes: completed, totalBytes: total,
                                          message: "Copying \(file.relativePath.contains("titanic1/") ? "Disc 1" : "Disc 2")…"))
                        lastProgress = Date()
                    }
                }
                try input.close()
                try writer.close()
                let after = try file.source.resourceValues(forKeys: [.fileSizeKey, .contentModificationDateKey])
                guard copied == file.size, Int64(after.fileSize ?? -1) == file.size,
                      after.contentModificationDate == file.modified else { throw ImportError.changedFile(file.source.lastPathComponent) }
                manifest[file.relativePath] = copied
            } catch {
                try? input.close()
                try? writer.close()
                throw error
            }
        }
        progress(Progress(completedBytes: completed, totalBytes: total, message: "Verifying your game files…"))
        try JSONSerialization.data(withJSONObject: manifest, options: [.sortedKeys]).write(to: staging.appendingPathComponent("gamefiles.json"), options: [.atomic])
        guard Self.isInstalled(at: staging) else { throw ImportError.invalidManifest }
        for (relative, expected) in manifest {
            try cancellation.check()
            let actual = try staging.appendingPathComponent(relative).resourceValues(forKeys: [.fileSizeKey]).fileSize
            guard Int64(actual ?? -1) == expected else { throw ImportError.invalidManifest }
        }
        let receipt: [String: Any] = ["importedAt": ISO8601DateFormatter().string(from: Date()),
                                      "files": files.count, "bytes": total, "edition": "en", "volumes": 2]
        try JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys]).write(to: staging.appendingPathComponent("Import.json"), options: [.atomic])
        try cancellation.check()
        let status: Int32
        if manager.fileExists(atPath: destination.path) {
            status = staging.path.withCString { source in
                destination.path.withCString { target in renameatx_np(AT_FDCWD, source, AT_FDCWD, target, UInt32(RENAME_SWAP)) }
            }
        } else {
            status = staging.path.withCString { source in destination.path.withCString { target in rename(source, target) } }
        }
        guard status == 0 else { throw ImportError.publishFailed(errno) }
        progress(Progress(completedBytes: total, totalBytes: total, message: "Ready to play."))
        return Result(directory: destination, files: files.count, bytes: total)
    }

    private func discoverDiscs(_ selections: [URL], cancellation: Cancellation) throws -> [Int: URL] {
        var candidates: [URL] = []
        let manager = FileManager.default
        func add(_ directory: URL, depth: Int) throws {
            try cancellation.check()
            let values = try directory.resourceValues(forKeys: [.isDirectoryKey, .isSymbolicLinkKey])
            guard values.isDirectory == true, values.isSymbolicLink != true else { return }
            candidates.append(directory)
            guard depth > 0 else { return }
            for child in try manager.contentsOfDirectory(at: directory, includingPropertiesForKeys: [.isDirectoryKey], options: [.skipsHiddenFiles]) {
                let name = child.lastPathComponent.lowercased()
                if ["cd1", "cd2", "disc1", "disc2", "disc 1", "disc 2", "titanic1", "titanic2", "discs", "gamefiles", "en"].contains(name) {
                    try add(child, depth: depth - 1)
                }
            }
        }
        for selection in selections { try add(selection, depth: 4) }
        var result: [Int: URL] = [:]
        for candidate in candidates {
            guard let dataFolder = try manager.contentsOfDirectory(at: candidate, includingPropertiesForKeys: nil, options: [.skipsHiddenFiles])
                .first(where: { $0.lastPathComponent.lowercased() == "data" }) else { continue }
            let filenames = Set((try? manager.contentsOfDirectory(atPath: dataFolder.path))?.map { $0.lowercased() } ?? [])
            if filenames.contains("bootfile") && filenames.contains("bedsit1.set") { result[1] = result[1] ?? candidate }
            if filenames.contains("a14.set") && filenames.contains("cargo.set") { result[2] = result[2] ?? candidate }
        }
        guard result[1] != nil, result[2] != nil, result[1] != result[2] else { throw ImportError.missingDiscs }
        return result
    }

    private func inventory(disc: Int, root: URL, cancellation: Cancellation) throws -> [SourceFile] {
        var result: [SourceFile] = []
        let keys: Set<URLResourceKey> = [.isRegularFileKey, .isDirectoryKey, .isSymbolicLinkKey, .fileSizeKey, .contentModificationDateKey]
        var enumerationError: Error?
        guard let walker = FileManager.default.enumerator(at: root, includingPropertiesForKeys: Array(keys), options: [.skipsHiddenFiles], errorHandler: { _, error in
            enumerationError = error
            return false
        }) else { throw ImportError.incompleteDisc(disc, "the folder could not be read") }
        for case let file as URL in walker {
            try cancellation.check()
            let values = try file.resourceValues(forKeys: keys)
            if values.isDirectory == true && Self.excludedFolders.contains(file.lastPathComponent.lowercased()) {
                walker.skipDescendants()
                continue
            }
            guard values.isSymbolicLink != true else { throw ImportError.unsafeFile(file.lastPathComponent) }
            guard values.isRegularFile == true else { continue }
            let relative = String(file.path.dropFirst(root.path.count + 1))
            guard Self.mediaExtensions.contains(file.pathExtension.lowercased()) || relative.lowercased() == "data/bootfile" else { continue }
            let size = Int64(values.fileSize ?? 0)
            guard size > 0 else { throw ImportError.incompleteDisc(disc, "\(relative) is empty") }
            result.append(SourceFile(source: file, relativePath: "gamefiles/en/titanic\(disc)/" + relative,
                                     size: size, modified: values.contentModificationDate))
        }
        if let enumerationError { throw enumerationError }
        return result.sorted { $0.relativePath < $1.relativePath }
    }

    private func validate(disc: Int, files: [SourceFile]) throws {
        let paths = Set(files.map { $0.relativePath.lowercased() })
        let prefix = "gamefiles/en/titanic\(disc)/"
        for needed in GameFileIndex.requiredPaths[disc]!.sorted() where !paths.contains(prefix + needed) {
            throw ImportError.incompleteDisc(disc, "\(needed) is missing")
        }
        for relative in Self.required[disc]! {
            guard let file = files.first(where: { $0.relativePath.lowercased() == prefix + relative }) else { continue }
            let handle = try FileHandle(forReadingFrom: file.source)
            defer { try? handle.close() }
            guard file.size >= 1024, let header = try handle.read(upToCount: 32), header.count == 32 else {
                throw ImportError.incompleteDisc(disc, "\(relative) has a damaged header")
            }
            func word(_ offset: Int, littleEndian: Bool) -> UInt32 {
                (0..<4).reduce(UInt32(0)) { value, index in
                    value | UInt32(header[offset + index]) << UInt32((littleEndian ? index : 3 - index) * 8)
                }
            }
            let valid = [true, false].contains { littleEndian in
                let declared = word(4, littleEndian: littleEndian)
                let containers = word(20, littleEndian: littleEndian)
                return Int64(declared) == file.size && containers > 0 && Int64(containers) * 4 + 1024 <= file.size
            }
            guard valid else { throw ImportError.incompleteDisc(disc, "\(relative) has a damaged or truncated container") }
        }
    }
}

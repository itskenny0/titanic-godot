import Foundation
import CryptoKit
import Darwin

/// Preserves original save bytes and imports them into the game's one writable
/// directory. There is deliberately no timestamp-based two-way synchronization.
struct SaveStore {
    let canonicalDirectory: URL
    let archivesDirectory: URL
    var revealURL: URL { canonicalDirectory }

    struct SaveSummary: Codable {
        let name: String
        let size: Int
        /// Unix milliseconds, suitable for JavaScript's Date constructor.
        let modifiedAt: Double
    }

    struct Record: Codable {
        let source: String
        let destination: String?
        let sha256: String?
        let disposition: String
        let detail: String?
    }

    struct Report: Codable {
        var records: [Record] = []
        var importedCount: Int { records.filter { $0.disposition == "imported" }.count }
        var archivedCount: Int { records.filter { $0.disposition == "archived" }.count }
        var skippedCount: Int { records.filter { $0.disposition == "skipped" }.count }
    }

    private struct StableSave {
        let data: Data
        let hash: String
        let modificationDate: Date?
    }

    enum StoreError: LocalizedError {
        case changedDuringRead(String)
        case archiveConflict(String)
        case invalidName
        case alreadyExists(String)
        case emptySave

        var errorDescription: String? {
            switch self {
            case .changedDuringRead(let path):
                return "The save is still changing and will be backed up on the next pass: \(path)"
            case .archiveConflict(let path):
                return "An existing save archive has unexpected content: \(path)"
            case .invalidName:
                return "Choose a save name without folder separators."
            case .alreadyExists(let name):
                return "A save named \(name) already exists."
            case .emptySave:
                return "The game supplied an empty save. Your existing save has been preserved."
            }
        }
    }

    func listSaves() throws -> [SaveSummary] {
        try makeDirectories()
        return try saveFiles(in: canonicalDirectory).map(summary)
    }

    func readSave(name: String) throws -> Data {
        try stableRead(validatedURL(name: name)).data
    }

    /// A requested replacement is reversible: archive the complete previous
    /// version first, then atomically replace it and archive the new version.
    func writeSave(name: String, data: Data, overwrite: Bool = false) throws -> SaveSummary {
        guard !data.isEmpty else { throw StoreError.emptySave }
        try makeDirectories()
        let destination = try validatedURL(name: name, addExtension: true)
        let occupied = try saveFiles(in: canonicalDirectory).first {
            $0.lastPathComponent.caseInsensitiveCompare(destination.lastPathComponent) == .orderedSame
        }
        let target = occupied ?? destination
        if let existing = occupied {
            _ = try validatedURL(name: existing.lastPathComponent)
            guard overwrite else { throw StoreError.alreadyExists(existing.lastPathComponent) }
            try archive(stableRead(existing), source: existing)
            try data.write(to: target, options: [.atomic])
        } else {
            try data.write(to: target, options: [.withoutOverwriting])
        }
        try archive(stableRead(target), source: target)
        return try summary(target)
    }

    private func summary(_ file: URL) throws -> SaveSummary {
        let attributes = try FileManager.default.attributesOfItem(atPath: file.path)
        return SaveSummary(name: file.lastPathComponent, size: (attributes[.size] as? NSNumber)?.intValue ?? 0,
                           modifiedAt: ((attributes[.modificationDate] as? Date)?.timeIntervalSince1970 ?? 0) * 1000)
    }

    private func validatedURL(name: String, addExtension: Bool = false) throws -> URL {
        guard !name.isEmpty, !name.hasPrefix("."), !name.contains("/"), !name.contains("\\"),
              !name.contains(":"), !name.contains("\0"), name.utf8.count <= 240 else { throw StoreError.invalidName }
        let filename = addExtension && !(name as NSString).pathExtension.lowercased().elementsEqual("ti")
            ? name + ".ti" : name
        guard (filename as NSString).pathExtension.lowercased() == "ti" else { throw StoreError.invalidName }
        let file = canonicalDirectory.appendingPathComponent(filename)
        // Do not let a pre-existing symbolic link escape the save directory.
        guard file.resolvingSymlinksInPath().deletingLastPathComponent().standardizedFileURL ==
                canonicalDirectory.resolvingSymlinksInPath().standardizedFileURL else { throw StoreError.invalidName }
        return file
    }

    /// Existing canonical saves always win their names. Sources are considered
    /// in caller-supplied order; distinct same-name versions get their own names.
    func importSaves(sources: [URL]) throws -> Report {
        try makeDirectories()
        var report = try snapshot()
        var contentIndex: [String: URL] = [:]
        for file in try saveFiles(in: canonicalDirectory) {
            if let save = try? stableRead(file) { contentIndex[save.hash] = file }
        }

        for source in sources {
            guard FileManager.default.fileExists(atPath: source.path) else { continue }
            for file in try saveFiles(in: source) {
                do {
                    let save = try stableRead(file)
                    try archive(save, source: file)
                    if let existing = contentIndex[save.hash] {
                        report.records.append(Record(source: file.path, destination: existing.path,
                                                     sha256: save.hash, disposition: "already-present", detail: nil))
                        continue
                    }
                    var destination = try importDestination(for: file, hash: save.hash)
                    // O_EXCL semantics prevent replacement even if another writer
                    // creates a save after the directory inventory was read.
                    do {
                        try save.data.write(to: destination, options: [.withoutOverwriting])
                    } catch {
                        guard FileManager.default.fileExists(atPath: destination.path) else { throw error }
                        destination = try importDestination(for: file, hash: save.hash, forceRecoveredName: true)
                        try save.data.write(to: destination, options: [.withoutOverwriting])
                    }
                    if let date = save.modificationDate {
                        try? FileManager.default.setAttributes([.modificationDate: date], ofItemAtPath: destination.path)
                    }
                    contentIndex[save.hash] = destination
                    report.records.append(Record(source: file.path, destination: destination.path,
                                                 sha256: save.hash, disposition: "imported", detail: nil))
                } catch {
                    report.records.append(Record(source: file.path, destination: nil, sha256: nil,
                                                 disposition: "skipped", detail: error.localizedDescription))
                }
            }
        }
        try persistReport(report, operation: "import")
        return report
    }

    /// Safe to call periodically while the game runs: a file that changes while
    /// being read is deferred; existing hash-addressed archives are never replaced.
    func snapshot() throws -> Report {
        try makeDirectories()
        var report = Report()
        for file in try saveFiles(in: canonicalDirectory) {
            do {
                let save = try stableRead(file)
                let created = try archive(save, source: file)
                report.records.append(Record(source: file.path, destination: archiveURL(save.hash).path,
                                             sha256: save.hash,
                                             disposition: created ? "archived" : "already-archived", detail: nil))
            } catch {
                report.records.append(Record(source: file.path, destination: nil, sha256: nil,
                                             disposition: "skipped", detail: error.localizedDescription))
            }
        }
        // Avoid creating an unbounded stream of identical periodic reports.
        if report.archivedCount > 0 || report.skippedCount > 0 {
            try persistReport(report, operation: "snapshot")
        }
        return report
    }

    private func makeDirectories() throws {
        for directory in [canonicalDirectory, archivesDirectory] {
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        }
    }

    private func saveFiles(in source: URL) throws -> [URL] {
        let values = try source.resourceValues(forKeys: [.isDirectoryKey, .isRegularFileKey])
        let candidates = values.isDirectory == true
            ? try FileManager.default.contentsOfDirectory(at: source, includingPropertiesForKeys: [.isRegularFileKey],
                                                         options: [.skipsHiddenFiles])
            : [source]
        return try candidates.filter {
            guard $0.pathExtension.lowercased() == "ti", !$0.lastPathComponent.hasPrefix("._") else { return false }
            return try $0.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile == true
        }.sorted { $0.lastPathComponent.localizedStandardCompare($1.lastPathComponent) == .orderedAscending }
    }

    private func stableRead(_ file: URL) throws -> StableSave {
        let manager = FileManager.default
        func attributes() throws -> NSDictionary {
            let all = try manager.attributesOfItem(atPath: file.path)
            return NSDictionary(dictionary: all.filter {
                [.size, .modificationDate, .systemFileNumber, .systemNumber, .type].contains($0.key)
            })
        }
        let before = try attributes()
        let first = try Data(contentsOf: file)
        Thread.sleep(forTimeInterval: 0.03)
        let second = try Data(contentsOf: file)
        let after = try attributes()
        guard !first.isEmpty, before == after, first == second,
              (after[FileAttributeKey.size] as? NSNumber)?.intValue == second.count else {
            throw StoreError.changedDuringRead(file.path)
        }
        return StableSave(data: second, hash: digest(second),
                          modificationDate: after[FileAttributeKey.modificationDate] as? Date)
    }

    private func digest(_ data: Data) -> String {
        SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }

    private func archiveURL(_ hash: String) -> URL {
        archivesDirectory.appendingPathComponent(hash).appendingPathExtension("ti")
    }

    @discardableResult
    private func archive(_ save: StableSave, source: URL) throws -> Bool {
        let destination = archiveURL(save.hash)
        if FileManager.default.fileExists(atPath: destination.path) {
            guard digest(try Data(contentsOf: destination)) == save.hash else {
                throw StoreError.archiveConflict(destination.path)
            }
            return false
        }
        // Publish a complete archive using an exclusive hard link. A crash or
        // full disk can leave a staging file, never a partial hash-named backup.
        let staging = archivesDirectory.appendingPathComponent(".save-\(UUID().uuidString).tmp")
        defer { try? FileManager.default.removeItem(at: staging) }
        try save.data.write(to: staging, options: [.withoutOverwriting])
        try FileManager.default.setAttributes([.posixPermissions: 0o444], ofItemAtPath: staging.path)
        guard Darwin.link(staging.path, destination.path) == 0 else {
            let errorCode = errno
            if errorCode == EEXIST {
                guard digest(try Data(contentsOf: destination)) == save.hash else {
                    throw StoreError.archiveConflict(destination.path)
                }
                return false
            }
            throw NSError(domain: NSPOSIXErrorDomain, code: Int(errorCode))
        }
        let record = Record(source: source.path, destination: destination.path, sha256: save.hash,
                            disposition: "archived", detail: nil)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        try encoder.encode(record).write(to: destination.deletingPathExtension().appendingPathExtension("json"),
                                         options: [.withoutOverwriting])
        return true
    }

    private func importDestination(for source: URL, hash: String, forceRecoveredName: Bool = false) throws -> URL {
        let occupied = Set(try FileManager.default.contentsOfDirectory(atPath: canonicalDirectory.path)
            .map { $0.lowercased() })
        if !forceRecoveredName && !occupied.contains(source.lastPathComponent.lowercased()) {
            return canonicalDirectory.appendingPathComponent(source.lastPathComponent)
        }
        var stem = source.deletingPathExtension().lastPathComponent
        while stem.utf8.count > 140 { stem.removeLast() }
        let preferred = "\(stem) (Recovered \(hash.prefix(8))).ti"
        if !occupied.contains(preferred.lowercased()) { return canonicalDirectory.appendingPathComponent(preferred) }
        // A prefix collision or deliberately named existing save must not cause
        // replacement. The full hash is deterministic; the suffix handles an
        // existing unrelated file using even that complete name.
        let full = "\(stem) (Recovered \(hash)).ti"
        if !occupied.contains(full.lowercased()) { return canonicalDirectory.appendingPathComponent(full) }
        var counter = 2
        while occupied.contains("\(stem) (Recovered \(hash)-\(counter)).ti".lowercased()) { counter += 1 }
        return canonicalDirectory.appendingPathComponent("\(stem) (Recovered \(hash)-\(counter)).ti")
    }

    private func persistReport(_ report: Report, operation: String) throws {
        let reports = archivesDirectory.appendingPathComponent("Records", isDirectory: true)
        try FileManager.default.createDirectory(at: reports, withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let filename = "\(operation)-\(UInt64(Date().timeIntervalSince1970 * 1000))-\(UUID().uuidString).json"
        try encoder.encode(report).write(to: reports.appendingPathComponent(filename), options: [.withoutOverwriting])
    }
}

import AppKit
import Foundation
import UniformTypeIdentifiers
import WebKit

private let gameTitle = "Titanic: Adventure Out of Time"

@main
final class TitanicApplication: NSObject, NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var engineReady = false
    private var engineMenuItems: [NSMenuItem] = []
    private var fullscreenItem: NSMenuItem!
    private var pendingTermination = false
    private var pendingLoad: String?
    private var filesOpenedBeforeLaunch: [URL] = []
    private var activeGameRoot: URL?
    private var setupView: GameSetupView?
    private var importGameItem: NSMenuItem?
    private var importCancellation: GameImporter.Cancellation?
    private let gameImportQueue = DispatchQueue(label: "org.titanic.restoration.game-import", qos: .userInitiated)
    private let saveQueue = DispatchQueue(label: "org.titanic.restoration.saves", qos: .userInitiated)
    private let diagnosticsQueue = DispatchQueue(label: "org.titanic.restoration.diagnostics", qos: .utility)
    private let supportURL = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        .appendingPathComponent("Titanic Adventure Out of Time", isDirectory: true)
    private lazy var saves = SaveStore(canonicalDirectory: supportURL.appendingPathComponent("Saves", isDirectory: true),
                                       archivesDirectory: supportURL.appendingPathComponent("Save Archives", isDirectory: true))
    private lazy var diagnosticsURL = supportURL.appendingPathComponent("Diagnostics", isDirectory: true)
    private lazy var bundledGameURL = Bundle.main.resourceURL!.appendingPathComponent("Game", isDirectory: true)
    private lazy var installedGameURL = supportURL.appendingPathComponent("Game", isDirectory: true)
    private lazy var hasBundledGame = GameImporter.isInstalled(at: bundledGameURL)

    static func main() {
        let app = NSApplication.shared
        let delegate = TitanicApplication()
        app.delegate = delegate
        app.setActivationPolicy(.regular)
        app.run()
        withExtendedLifetime(delegate) {}
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        UserDefaults.standard.register(defaults: ["StartFullscreen": true])
        activeGameRoot = hasBundledGame ? bundledGameURL : (GameImporter.isInstalled(at: installedGameURL) ? installedGameURL : nil)
        buildMenu()
        buildWindow()
        prepareSaves()
        writeDiagnostic(["event": "launch", "version": Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") ?? "development",
                         "bundle": Bundle.main.bundleURL.path, "pid": ProcessInfo.processInfo.processIdentifier])
        if let activeGameRoot { startGame(at: activeGameRoot) }
        else { showGameSetup() }
        if !filesOpenedBeforeLaunch.isEmpty {
            let files = filesOpenedBeforeLaunch
            filesOpenedBeforeLaunch.removeAll()
            importOpenedFiles(files)
        }
        NSApp.activate(ignoringOtherApps: true)
    }

    private func enterPreferredFullscreen() {
        if UserDefaults.standard.bool(forKey: "StartFullscreen") && !window.styleMask.contains(.fullScreen) {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                guard let self, !self.window.styleMask.contains(.fullScreen) else { return }
                self.window.toggleFullScreen(nil)
            }
        }
    }

    private func buildWindow() {
        webView = makeWebView(gameRoot: activeGameRoot ?? installedGameURL)
        let available = NSScreen.main?.visibleFrame.size ?? NSSize(width: 1280, height: 900)
        let width = min(1024.0, max(640.0, available.width * 0.85))
        let height = min(width * 0.75, max(480.0, available.height * 0.85 - 28))
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: width, height: height),
                          styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = gameTitle
        window.backgroundColor = .black
        window.isOpaque = true
        window.contentMinSize = NSSize(width: 640, height: 480)
        window.collectionBehavior = [.fullScreenPrimary, .managed]
        window.tabbingMode = .disallowed
        window.isReleasedWhenClosed = false
        window.acceptsMouseMovedEvents = true
        window.delegate = self
        window.contentView = webView
        window.setFrameAutosaveName("TitanicGameWindow")
        if !window.setFrameUsingName("TitanicGameWindow") { window.center() }
        window.makeKeyAndOrderFront(nil)
        window.makeFirstResponder(webView)
    }

    private func makeWebView(gameRoot: URL) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.mediaTypesRequiringUserActionForPlayback = []
        configuration.preferences.javaScriptCanOpenWindowsAutomatically = false
        configuration.websiteDataStore = .default()
        configuration.userContentController.add(self, name: "titanicHost")
        let resources = Bundle.main.resourceURL!
        configuration.setURLSchemeHandler(GameSchemeHandler(webRoot: resources.appendingPathComponent("Web", isDirectory: true),
                                                              gameRoot: gameRoot),
                                          forURLScheme: "titanic")
        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.setValue(false, forKey: "drawsBackground")
        webView.allowsBackForwardNavigationGestures = false
        webView.allowsMagnification = false
        #if DEBUG
        if #available(macOS 13.3, *) { webView.isInspectable = true }
        #endif
        return webView
    }

    private func startGame(at root: URL) {
        engineReady = false
        engineMenuItems.forEach { $0.isEnabled = false }
        webView.stopLoading()
        webView.configuration.userContentController.removeScriptMessageHandler(forName: "titanicHost")
        webView.navigationDelegate = nil
        webView.uiDelegate = nil
        webView = makeWebView(gameRoot: root)
        activeGameRoot = root
        setupView = nil
        window.contentMinSize = NSSize(width: 640, height: 480)
        window.contentView = webView
        window.makeFirstResponder(webView)
        importGameItem?.isEnabled = true
        writeDiagnostic(["event": "gameFilesReady", "source": root == bundledGameURL ? "bundled" : "imported"])
        webView.load(URLRequest(url: URL(string: "titanic://app/index.html")!))
        enterPreferredFullscreen()
    }

    private func buildMenu() {
        let bar = NSMenu()
        let applicationMenu = NSMenu(title: "Titanic")
        addMenu(applicationMenu, to: bar)
        applicationMenu.addItem(withTitle: "About Titanic", action: #selector(showAbout), keyEquivalent: "")
        applicationMenu.addItem(.separator())
        applicationMenu.addItem(withTitle: "Hide Titanic", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        let hideOthers = applicationMenu.addItem(withTitle: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]
        applicationMenu.addItem(withTitle: "Show All", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
        applicationMenu.addItem(.separator())
        applicationMenu.addItem(withTitle: "Quit Titanic", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

        let gameMenu = NSMenu(title: "Game")
        addMenu(gameMenu, to: bar)
        addEngineCommand("New Game…", command: "newGame", key: "n", to: gameMenu)
        addEngineCommand("Load Game…", command: "load", key: "o", to: gameMenu)
        addEngineCommand("Save Game…", command: "save", key: "s", to: gameMenu)
        gameMenu.addItem(.separator())
        addAction("Import Saved Games…", action: #selector(importFromMenu), key: "i", to: gameMenu)
        addAction("Export Saved Game…", action: #selector(exportFromMenu), key: "e", to: gameMenu)
        addAction("Show Saved Games in Finder", action: #selector(revealSaves), key: "", to: gameMenu)
        if !hasBundledGame {
            gameMenu.addItem(.separator())
            importGameItem = addAction("Import Game Files…", action: #selector(importGameFilesFromMenu), key: "", to: gameMenu)
        }

        let edit = NSMenu(title: "Edit")
        addMenu(edit, to: bar)
        edit.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        edit.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        edit.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        edit.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

        let view = NSMenu(title: "View")
        addMenu(view, to: bar)
        fullscreenItem = addAction("Enter Full Screen", action: #selector(toggleFullscreen), key: "f", to: view)
        fullscreenItem.keyEquivalentModifierMask = [.command, .control]
        let windows = NSMenu(title: "Window")
        addMenu(windows, to: bar)
        windows.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        windows.addItem(withTitle: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        NSApp.windowsMenu = windows
        let help = NSMenu(title: "Help")
        addMenu(help, to: bar)
        addAction("Titanic Controls", action: #selector(showControls), key: "", to: help)
        addAction("Show Diagnostics in Finder", action: #selector(revealDiagnostics), key: "", to: help)
        NSApp.helpMenu = help
        NSApp.mainMenu = bar
    }

    private func addMenu(_ menu: NSMenu, to bar: NSMenu) {
        let item = NSMenuItem(title: menu.title, action: nil, keyEquivalent: "")
        item.submenu = menu
        bar.addItem(item)
    }

    @discardableResult
    private func addAction(_ title: String, action: Selector, key: String, to menu: NSMenu) -> NSMenuItem {
        let item = menu.addItem(withTitle: title, action: action, keyEquivalent: key)
        item.target = self
        return item
    }

    private func addEngineCommand(_ title: String, command: String, key: String, to menu: NSMenu) {
        let item = addAction(title, action: #selector(engineCommand(_:)), key: key, to: menu)
        item.representedObject = command
        item.isEnabled = false
        menu.autoenablesItems = false
        engineMenuItems.append(item)
    }

    @objc private func importGameFilesFromMenu() {
        guard importCancellation == nil else { return }
        showGameSetup()
    }

    private func showGameSetup() {
        sendCommand("pause")
        engineMenuItems.forEach { $0.isEnabled = false }
        let setup = GameSetupView(canReturnToGame: activeGameRoot != nil)
        setup.onChooseParent = { [weak self] in self?.chooseGameParent() }
        setup.onChooseDiscs = { [weak self] in self?.chooseGameDiscs() }
        setup.onCancel = { [weak self] in self?.importCancellation?.cancel() }
        setup.onReturn = { [weak self] in self?.returnFromGameSetup() }
        setupView = setup
        window.contentMinSize = NSSize(width: 640, height: 560)
        if let size = window.contentView?.frame.size, size.height < 560 && !window.styleMask.contains(.fullScreen) {
            window.setContentSize(NSSize(width: max(640, size.width), height: 560))
        }
        window.contentView = setup
        writeDiagnostic(["event": "gameSetup", "canReturnToGame": activeGameRoot != nil])
    }

    private func returnFromGameSetup() {
        guard activeGameRoot != nil, importCancellation == nil else { return }
        setupView = nil
        window.contentMinSize = NSSize(width: 640, height: 480)
        window.contentView = webView
        window.makeFirstResponder(webView)
        engineMenuItems.forEach { $0.isEnabled = engineReady }
        sendCommand("resume")
    }

    private func gameFolderPanel(title: String, message: String) -> NSOpenPanel {
        let panel = NSOpenPanel()
        panel.title = title
        panel.message = message
        panel.prompt = "Choose Folder"
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.canCreateDirectories = false
        return panel
    }

    private func chooseGameParent() {
        let panel = gameFolderPanel(title: "Choose Titanic Game Folder",
                                    message: "Choose the parent folder containing both extracted discs, such as a folder with cd1 and cd2 inside it.")
        panel.beginSheetModal(for: window) { [weak self] response in
            guard response == .OK, let folder = panel.url else { return }
            self?.beginGameImport([folder])
        }
    }

    private func chooseGameDiscs() {
        let first = gameFolderPanel(title: "Choose Titanic Disc 1",
                                    message: "Select the extracted Disc 1 folder containing DATA, MOVIES, and PUPPETS2.")
        first.beginSheetModal(for: window) { [weak self] response in
            guard let self, response == .OK, let disc1 = first.url else { return }
            let second = self.gameFolderPanel(title: "Choose Titanic Disc 2",
                                               message: "Select the extracted Disc 2 folder containing DATA, MOVIES, and PUPPETS1.")
            second.directoryURL = disc1.deletingLastPathComponent()
            second.beginSheetModal(for: self.window) { [weak self] response in
                guard response == .OK, let disc2 = second.url else { return }
                self?.beginGameImport([disc1, disc2])
            }
        }
    }

    private func beginGameImport(_ selections: [URL]) {
        guard importCancellation == nil else { return }
        let cancellation = GameImporter.Cancellation()
        importCancellation = cancellation
        importGameItem?.isEnabled = false
        setupView?.showProgress(.init(completedBytes: 0, totalBytes: 0, message: "Checking both discs…"))
        let destination = installedGameURL
        gameImportQueue.async { [weak self] in
            let accessed = selections.filter { $0.startAccessingSecurityScopedResource() }
            defer { accessed.forEach { $0.stopAccessingSecurityScopedResource() } }
            do {
                let result = try GameImporter().importDiscs(from: selections, to: destination, cancellation: cancellation) { state in
                    DispatchQueue.main.async { self?.setupView?.showProgress(state) }
                }
                DispatchQueue.main.async {
                    guard let self else { return }
                    self.importCancellation = nil
                    self.writeDiagnostic(["event": "gameImportComplete", "files": result.files, "bytes": result.bytes])
                    if !self.pendingTermination { self.startGame(at: result.directory) }
                }
            } catch {
                DispatchQueue.main.async {
                    guard let self else { return }
                    self.importCancellation = nil
                    self.importGameItem?.isEnabled = true
                    self.writeDiagnostic(["event": "gameImportError", "message": error.localizedDescription])
                    if !self.pendingTermination { self.setupView?.showError(error) }
                }
            }
        }
    }

    private func prepareSaves() {
        let store = saves
        let initial = Bundle.main.resourceURL!.appendingPathComponent("Initial Saves", isDirectory: true)
        let marker = supportURL.appendingPathComponent("initial-saves-imported.json")
        saveQueue.async { [weak self] in
            do {
                guard FileManager.default.fileExists(atPath: initial.path), !FileManager.default.fileExists(atPath: marker.path) else {
                    _ = try store.snapshot()
                    return
                }
                let report = try store.importSaves(sources: [initial])
                // Deleting a seed save is a persistent user choice. Only retry
                // the initial import when a source could not yet be read.
                if report.skippedCount == 0 {
                    let receipt: [String: Any] = ["completedAt": ISO8601DateFormatter().string(from: Date()),
                                                  "importedCount": report.importedCount]
                    try JSONSerialization.data(withJSONObject: receipt, options: [.sortedKeys]).write(to: marker, options: [.atomic])
                }
                self?.writeDiagnostic(["event": "saveImport", "imported": report.importedCount, "archived": report.archivedCount,
                                       "skipped": report.skippedCount])
            } catch {
                self?.writeDiagnostic(["event": "saveImportError", "message": error.localizedDescription])
                DispatchQueue.main.async { [weak self] in self?.showNote(title: "Saved games could not be prepared", message: error.localizedDescription) }
            }
        }
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.frameInfo.isMainFrame, message.frameInfo.request.url?.scheme == "titanic",
              message.frameInfo.request.url?.host == "app", let body = message.body as? [String: Any],
              let action = body["action"] as? String else { return }
        let id = body["id"]
        switch action {
        case "ready":
            engineReady = true
            engineMenuItems.forEach { $0.isEnabled = setupView == nil }
            if !NSApp.isActive || setupView != nil { sendCommand("pause") }
            writeDiagnostic(body.merging(["event": "ready"]) { _, new in new })
            reply(id, result: ["platform": "macOS", "fullscreen": window.styleMask.contains(.fullScreen)])
        case "log", "diagnostics":
            writeDiagnostic(body, snapshot: action == "diagnostics")
            reply(id, result: NSNull())
        case "setPendingLoad":
            guard let encoded = body["data"] as? String, encoded.count <= 32 * 1024 * 1024,
                  let data = Data(base64Encoded: encoded), !data.isEmpty else {
                reply(id, error: "The saved game could not be prepared for loading."); return
            }
            pendingLoad = encoded
            reply(id, result: NSNull())
        case "takePendingLoad":
            let encoded = pendingLoad
            pendingLoad = nil
            reply(id, result: encoded.map { ["data": $0] } as Any? ?? NSNull())
        case "listSaves":
            performSaveWork(id) { try self.jsonObject(self.saves.listSaves()) }
        case "readSave":
            guard let name = body["name"] as? String else { reply(id, error: "A save name is required."); return }
            performSaveWork(id) { ["name": name, "data": try self.saves.readSave(name: name).base64EncodedString()] }
        case "writeSave":
            guard let name = body["name"] as? String, let encoded = body["data"] as? String,
                  encoded.count <= 32 * 1024 * 1024, let data = Data(base64Encoded: encoded), !data.isEmpty else {
                reply(id, error: "The game did not supply a valid saved game."); return
            }
            let overwrite = body["overwrite"] as? Bool ?? false
            performSaveWork(id) {
                let summary = try self.saves.writeSave(name: name, data: data, overwrite: overwrite)
                self.writeDiagnostic(["event": "saveWritten", "name": summary.name, "size": summary.size])
                return try self.jsonObject(summary)
            }
        case "chooseSave": chooseSave { [weak self] result, error in self?.reply(id, result: result, error: error) }
        case "importSaves": importSaves { [weak self] result, error in self?.reply(id, result: result, error: error) }
        case "revealSaves": revealSaves(); reply(id, result: NSNull())
        case "setFullscreen":
            let desired = body["fullscreen"] as? Bool ?? !window.styleMask.contains(.fullScreen)
            if desired != window.styleMask.contains(.fullScreen) { window.toggleFullScreen(nil) }
            reply(id, result: ["fullscreen": desired])
        case "noteDialog":
            showNote(title: body["title"] as? String ?? gameTitle, message: body["message"] as? String ?? "") { [weak self] in
                self?.reply(id, result: NSNull())
            }
        case "questionDialog":
            showQuestion(body) { [weak self] index in self?.reply(id, result: ["index": index, "accepted": index == 0]) }
        case "textDialog":
            showTextDialog(body) { [weak self] value in self?.reply(id, result: ["text": value as Any? ?? NSNull()]) }
        case "quit": reply(id, result: NSNull()); NSApp.terminate(nil)
        default: reply(id, error: "Unknown native action: \(action)")
        }
    }

    private func performSaveWork(_ id: Any?, operation: @escaping () throws -> Any) {
        saveQueue.async { [weak self] in
            do {
                let result = try operation()
                DispatchQueue.main.async { self?.reply(id, result: result) }
            } catch {
                let description = error.localizedDescription
                self?.writeDiagnostic(["event": "saveError", "message": description])
                DispatchQueue.main.async { self?.reply(id, error: description) }
            }
        }
    }

    private func jsonObject<T: Encodable>(_ value: T) throws -> Any {
        try JSONSerialization.jsonObject(with: JSONEncoder().encode(value))
    }

    private func reply(_ id: Any?, result: Any? = nil, error: String? = nil) {
        guard let id else { return }
        var payload: [String: Any] = ["id": id, "ok": error == nil]
        if let error { payload["error"] = error }
        else { payload["result"] = result ?? NSNull() }
        evaluateBridge("receive", payload: payload)
    }

    private func evaluateBridge(_ method: String, payload: [String: Any]) {
        guard let data = try? JSONSerialization.data(withJSONObject: payload, options: [.fragmentsAllowed]),
              let json = String(data: data, encoding: .utf8) else { return }
        webView.evaluateJavaScript("window.titanicHost?.\(method)?.(\(json));") { [weak self] _, error in
            if let error { self?.writeDiagnostic(["event": "bridgeError", "method": method, "message": error.localizedDescription]) }
        }
    }

    private func sendCommand(_ command: String) {
        guard engineReady else { return }
        evaluateBridge("onCommand", payload: ["action": command])
    }

    @objc private func engineCommand(_ sender: NSMenuItem) {
        if let command = sender.representedObject as? String { sendCommand(command) }
    }

    private func savePanel(title: String, multiple: Bool) -> NSOpenPanel {
        let panel = NSOpenPanel()
        panel.title = title
        panel.prompt = multiple ? "Import" : "Load Game"
        panel.allowedContentTypes = [UTType(filenameExtension: "ti") ?? .data]
        panel.allowsOtherFileTypes = false
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = multiple
        panel.directoryURL = saves.revealURL
        return panel
    }

    private func chooseSave(completion: @escaping (Any?, String?) -> Void) {
        let panel = savePanel(title: "Load Titanic Saved Game", multiple: false)
        panel.beginSheetModal(for: window) { [weak self] response in
            guard let self else { return }
            guard response == .OK, let file = panel.url else { completion(NSNull(), nil); return }
            self.saveQueue.async {
                do {
                    let report = try self.saves.importSaves(sources: [file])
                    guard let record = report.records.last(where: { $0.source == file.path && $0.destination != nil }),
                          let destination = record.destination else {
                        throw NSError(domain: "TitanicSaves", code: 1,
                                      userInfo: [NSLocalizedDescriptionKey: report.records.last?.detail ?? "This saved game could not be read."])
                    }
                    let name = URL(fileURLWithPath: destination).lastPathComponent
                    let data = try self.saves.readSave(name: name)
                    self.writeDiagnostic(["event": "saveChosen", "name": name, "size": data.count])
                    DispatchQueue.main.async { completion(["name": name, "data": data.base64EncodedString()], nil) }
                } catch {
                    let description = error.localizedDescription
                    DispatchQueue.main.async { completion(nil, description) }
                }
            }
        }
    }

    private func importSaves(completion: @escaping (Any?, String?) -> Void) {
        let panel = savePanel(title: "Import Titanic Saved Games", multiple: true)
        panel.beginSheetModal(for: window) { [weak self] response in
            guard let self else { return }
            guard response == .OK else { completion(NSNull(), nil); return }
            let files = panel.urls
            self.saveQueue.async {
                do {
                    let report = try self.saves.importSaves(sources: files)
                    let result: [String: Any] = ["importedCount": report.importedCount, "skippedCount": report.skippedCount,
                                                  "saves": try self.jsonObject(self.saves.listSaves())]
                    DispatchQueue.main.async { completion(result, nil) }
                } catch {
                    let description = error.localizedDescription
                    DispatchQueue.main.async { completion(nil, description) }
                }
            }
        }
    }

    @objc private func importFromMenu() {
        importSaves { [weak self] result, error in
            guard let self else { return }
            if let error { self.showNote(title: "Saved games could not be imported", message: error); return }
            guard let result = result as? [String: Any], let imported = result["importedCount"] as? Int else { return }
            let skipped = result["skippedCount"] as? Int ?? 0
            self.showNote(title: "Saved games imported", message: "\(imported) saved game\(imported == 1 ? "" : "s") added."
                          + (skipped > 0 ? " \(skipped) could not be read; see the save archive records for details." : " You can load them from Game → Load Game."))
        }
    }

    @objc private func exportFromMenu() {
        let picker = savePanel(title: "Choose Saved Game to Export", multiple: false)
        picker.prompt = "Choose"
        picker.beginSheetModal(for: window) { [weak self] response in
            guard let self, response == .OK, let source = picker.url else { return }
            let panel = NSSavePanel()
            panel.title = "Export Titanic Saved Game"
            panel.nameFieldStringValue = source.lastPathComponent
            panel.allowedContentTypes = [UTType(filenameExtension: "ti") ?? .data]
            panel.canCreateDirectories = true
            panel.beginSheetModal(for: self.window) { response in
                guard response == .OK, let destination = panel.url else { return }
                self.saveQueue.async {
                    do {
                        let bytes = try Data(contentsOf: source)
                        try bytes.write(to: destination, options: [.atomic])
                        self.writeDiagnostic(["event": "saveExported", "name": source.lastPathComponent, "size": bytes.count])
                    } catch {
                        let description = error.localizedDescription
                        DispatchQueue.main.async { self.showNote(title: "Saved game could not be exported", message: description) }
                    }
                }
            }
        }
    }

    @objc private func revealSaves() {
        try? FileManager.default.createDirectory(at: saves.revealURL, withIntermediateDirectories: true)
        NSWorkspace.shared.open(saves.revealURL)
    }

    @objc private func revealDiagnostics() {
        try? FileManager.default.createDirectory(at: diagnosticsURL, withIntermediateDirectories: true)
        NSWorkspace.shared.open(diagnosticsURL)
    }

    @objc private func toggleFullscreen() { window.toggleFullScreen(nil) }

    func windowDidEnterFullScreen(_ notification: Notification) {
        UserDefaults.standard.set(true, forKey: "StartFullscreen")
        fullscreenItem.title = "Exit Full Screen"
        evaluateBridge("onFullscreenChanged", payload: ["fullscreen": true])
        sendCommand("resumeAudio")
    }

    func windowDidExitFullScreen(_ notification: Notification) {
        UserDefaults.standard.set(false, forKey: "StartFullscreen")
        fullscreenItem.title = "Enter Full Screen"
        evaluateBridge("onFullscreenChanged", payload: ["fullscreen": false])
        sendCommand("resumeAudio")
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    func applicationDidResignActive(_ notification: Notification) { sendCommand("pause") }

    func applicationDidBecomeActive(_ notification: Notification) { if setupView == nil { sendCommand("resume") } }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        if pendingTermination { return .terminateLater }
        pendingTermination = true
        importCancellation?.cancel()
        writeDiagnostic(["event": "quit"])
        // Drain queued atomic save writes and their backups before ending the process.
        gameImportQueue.async { [weak self] in
            self?.saveQueue.async {
                self?.diagnosticsQueue.async {
                    DispatchQueue.main.async { NSApp.reply(toApplicationShouldTerminate: true) }
                }
            }
        }
        return .terminateLater
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        window.makeKeyAndOrderFront(nil)
        return true
    }

    func application(_ sender: NSApplication, openFiles filenames: [String]) {
        let files = filenames.map { URL(fileURLWithPath: $0) }
        guard window != nil else { filesOpenedBeforeLaunch.append(contentsOf: files); return }
        importOpenedFiles(files)
    }

    private func importOpenedFiles(_ files: [URL]) {
        let store = saves
        saveQueue.async { [weak self] in
            guard let self else { return }
            do {
                let report = try store.importSaves(sources: files)
                if files.count == 1, let file = files.first {
                    guard let destination = report.records.last(where: { $0.source == file.path && $0.destination != nil })?.destination else {
                        throw NSError(domain: "TitanicSaves", code: 1, userInfo: [NSLocalizedDescriptionKey:
                            report.records.last?.detail ?? "This saved game could not be imported."])
                    }
                    let name = URL(fileURLWithPath: destination).lastPathComponent
                    let data = try store.readSave(name: name)
                    DispatchQueue.main.async {
                        self.pendingLoad = data.base64EncodedString()
                        self.writeDiagnostic(["event": "saveOpenedFromFinder", "name": name, "size": data.count])
                        // Let the current engine validate first so a damaged
                        // Finder document cannot discard the active voyage.
                        // During startup, boot consumes and validates this data.
                        self.sendCommand("loadOpenedSave")
                        self.window.makeKeyAndOrderFront(nil)
                        NSApp.activate(ignoringOtherApps: true)
                        NSApp.reply(toOpenOrPrint: .success)
                    }
                } else {
                    DispatchQueue.main.async {
                        self.showNote(title: "Saved games imported", message: "Your selected saved games are available from Game → Load Game."
                                      + (report.skippedCount > 0 ? " \(report.skippedCount) could not be read." : ""))
                        NSApp.reply(toOpenOrPrint: report.skippedCount == files.count ? .failure : .success)
                    }
                }
            } catch {
                let description = error.localizedDescription
                DispatchQueue.main.async {
                    self.showNote(title: "Saved game could not be opened", message: description)
                    NSApp.reply(toOpenOrPrint: .failure)
                }
            }
        }
    }

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
                 decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = navigationAction.request.url else { decisionHandler(.cancel); return }
        decisionHandler(url.scheme == "titanic" && url.host == "app" ? .allow : .cancel)
    }

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
        engineReady = false
        engineMenuItems.forEach { $0.isEnabled = false }
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        showStartupFailure(error)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { showStartupFailure(error) }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        engineReady = false
        engineMenuItems.forEach { $0.isEnabled = false }
        writeDiagnostic(["event": "webProcessTerminated"])
        showQuestion(["title": "Titanic stopped unexpectedly", "message": "Your saved games are safe. Reload the game to continue.",
                      "buttons": ["Reload Game", "Quit"]]) { [weak self] index in
            if index == 0 { self?.webView.reload() } else { NSApp.terminate(nil) }
        }
    }

    private func showStartupFailure(_ error: Error) {
        guard (error as NSError).code != NSURLErrorCancelled else { return }
        writeDiagnostic(["event": "navigationError", "message": error.localizedDescription])
        showNote(title: "Titanic could not start", message: error.localizedDescription)
    }

    private func showNote(title: String, message: String, completion: (() -> Void)? = nil) {
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = message
        alert.addButton(withTitle: "OK")
        alert.beginSheetModal(for: window) { _ in completion?() }
    }

    private func showQuestion(_ body: [String: Any], completion: @escaping (Int) -> Void) {
        let alert = NSAlert()
        alert.messageText = body["title"] as? String ?? gameTitle
        alert.informativeText = body["message"] as? String ?? ""
        let buttons = body["buttons"] as? [String] ?? ["OK", "Cancel"]
        for title in buttons.prefix(4) { alert.addButton(withTitle: title) }
        if alert.buttons.isEmpty { alert.addButton(withTitle: "OK") }
        if alert.buttons.count > 1 { alert.buttons.last?.keyEquivalent = "\u{1b}" }
        alert.beginSheetModal(for: window) { response in completion(max(0, response.rawValue - NSApplication.ModalResponse.alertFirstButtonReturn.rawValue)) }
    }

    private func showTextDialog(_ body: [String: Any], completion: @escaping (String?) -> Void) {
        let alert = NSAlert()
        alert.messageText = body["title"] as? String ?? "Save Game"
        alert.informativeText = body["message"] as? String ?? "Choose a name for this saved game."
        alert.addButton(withTitle: body["confirmLabel"] as? String ?? "Save")
        alert.addButton(withTitle: "Cancel").keyEquivalent = "\u{1b}"
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 320, height: 24))
        field.stringValue = body["defaultValue"] as? String ?? ""
        field.placeholderString = body["placeholder"] as? String
        alert.accessoryView = field
        alert.window.initialFirstResponder = field
        alert.beginSheetModal(for: window) { response in
            completion(response == .alertFirstButtonReturn ? field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines) : nil)
        }
        field.selectText(nil)
    }

    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        showNote(title: gameTitle, message: message, completion: completionHandler)
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        showQuestion(["message": message]) { completionHandler($0 == 0) }
    }

    func webView(_ webView: WKWebView, runJavaScriptTextInputPanelWithPrompt prompt: String, defaultText: String?,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (String?) -> Void) {
        showTextDialog(["message": prompt, "defaultValue": defaultText ?? "", "confirmLabel": "OK"], completion: completionHandler)
    }

    @objc private func showAbout() {
        NSApp.orderFrontStandardAboutPanel(options: [.applicationName: gameTitle,
            .applicationVersion: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "1.0",
            .credits: NSAttributedString(string: "A local macOS restoration using the DreamRefactory engine and your original game media.\n\nOriginal game © CyberFlix. Engine and restoration notices are included inside the app.")])
    }

    @objc private func showControls() {
        showNote(title: "Titanic Controls", message: "Use the pointer to explore, talk, and interact. Use the game’s original on-screen controls for movement, inventory, and menus.\n\n⌘S  Save game\n⌘O  Load game\n⌃⌘F  Enter or leave full screen\n⌘Q  Quit\n\nBoth discs are included and available automatically. Saved games and their backups stay in your Library/Application Support/Titanic Adventure Out of Time folder.")
    }

    private func writeDiagnostic(_ payload: [String: Any], snapshot: Bool = false) {
        let directory = diagnosticsURL
        var record = payload
        record["timestamp"] = ISO8601DateFormatter().string(from: Date())
        guard JSONSerialization.isValidJSONObject(record), let data = try? JSONSerialization.data(withJSONObject: record, options: [.sortedKeys]) else { return }
        diagnosticsQueue.async {
            do {
                try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
                if snapshot { try data.write(to: directory.appendingPathComponent("latest.json"), options: [.atomic]); return }
                let log = directory.appendingPathComponent("session.jsonl")
                if let size = try? log.resourceValues(forKeys: [.fileSizeKey]).fileSize, size > 8 * 1024 * 1024 {
                    let previous = directory.appendingPathComponent("previous-session.jsonl")
                    try? FileManager.default.removeItem(at: previous)
                    try FileManager.default.moveItem(at: log, to: previous)
                }
                if !FileManager.default.fileExists(atPath: log.path) { FileManager.default.createFile(atPath: log.path, contents: nil) }
                let handle = try FileHandle(forWritingTo: log)
                defer { try? handle.close() }
                try handle.seekToEnd()
                try handle.write(contentsOf: data + Data([0x0a]))
            } catch { NSLog("Titanic diagnostics: %@", error.localizedDescription) }
        }
    }
}

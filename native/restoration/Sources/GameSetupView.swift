import AppKit

/// The public app's one-time setup is native, keyboard accessible, and separate
/// from the game canvas. Owned game files stay outside the signed application.
final class GameSetupView: NSView {
    var onChooseParent: (() -> Void)?
    var onChooseDiscs: (() -> Void)?
    var onCancel: (() -> Void)?
    var onReturn: (() -> Void)?

    private let message = NSTextField(wrappingLabelWithString: "Bring your original game back aboard. Choose the folder containing both extracted English game discs. Titanic will copy the files once, then play completely offline.")
    private let detail = NSTextField(wrappingLabelWithString: "Original game files are required and are not included with this app.")
    private let progress = NSProgressIndicator()
    private let choose = NSButton(title: "Choose Game Folder…", target: nil, action: nil)
    private let separate = NSButton(title: "Choose Discs Separately…", target: nil, action: nil)
    private let cancel = NSButton(title: "Cancel Import", target: nil, action: nil)
    private let back = NSButton(title: "Return to Game", target: nil, action: nil)
    private var importing = false

    init(canReturnToGame: Bool) {
        super.init(frame: .zero)
        wantsLayer = true
        let gold = NSColor(calibratedRed: 0.84, green: 0.73, blue: 0.50, alpha: 1)
        let heading = NSTextField(labelWithString: "TITANIC")
        heading.font = NSFont(name: "Baskerville", size: 46) ?? .systemFont(ofSize: 42, weight: .light)
        heading.textColor = gold
        heading.alignment = .center
        let subtitle = NSTextField(labelWithString: "A D V E N T U R E   O U T   O F   T I M E")
        subtitle.font = .systemFont(ofSize: 11, weight: .medium)
        subtitle.textColor = gold.withAlphaComponent(0.8)
        subtitle.alignment = .center
        let title = NSTextField(labelWithString: canReturnToGame ? "Import game files" : "Welcome aboard")
        title.font = .systemFont(ofSize: 22, weight: .semibold)
        title.textColor = .white
        title.alignment = .center
        for field in [message, detail] {
            field.alignment = .center
            field.maximumNumberOfLines = 0
            field.translatesAutoresizingMaskIntoConstraints = false
            field.setContentCompressionResistancePriority(.required, for: .vertical)
        }
        message.font = .systemFont(ofSize: 14)
        message.textColor = NSColor(white: 0.8, alpha: 1)
        detail.font = .systemFont(ofSize: 12)
        detail.textColor = NSColor(white: 0.58, alpha: 1)
        if canReturnToGame {
            message.stringValue = "Choose both extracted English game discs. Your current files stay available until the new import is complete. The game restarts when the new files are ready."
        }
        choose.target = self
        choose.action = #selector(chooseParent)
        choose.bezelStyle = .rounded
        choose.controlSize = .large
        choose.keyEquivalent = "\r"
        separate.target = self
        separate.action = #selector(chooseSeparately)
        separate.bezelStyle = .rounded
        cancel.target = self
        cancel.action = #selector(cancelImport)
        cancel.bezelStyle = .rounded
        cancel.isHidden = true
        cancel.keyEquivalent = "\u{1b}"
        back.target = self
        back.action = #selector(returnToGame)
        back.bezelStyle = .rounded
        back.isHidden = !canReturnToGame
        progress.style = .bar
        progress.minValue = 0
        progress.maxValue = 1
        progress.isIndeterminate = true
        progress.isHidden = true
        progress.translatesAutoresizingMaskIntoConstraints = false
        let buttons = NSStackView(views: [choose, separate])
        buttons.orientation = .vertical
        buttons.spacing = 10
        buttons.alignment = .centerX
        let stack = NSStackView(views: [heading, subtitle, title, message, progress, detail, buttons, cancel, back])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 18
        stack.setCustomSpacing(4, after: heading)
        stack.setCustomSpacing(38, after: subtitle)
        stack.setCustomSpacing(12, after: title)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.centerXAnchor.constraint(equalTo: centerXAnchor),
            stack.centerYAnchor.constraint(equalTo: centerYAnchor),
            stack.widthAnchor.constraint(lessThanOrEqualToConstant: 500),
            stack.widthAnchor.constraint(equalTo: widthAnchor, constant: -100).withPriority(.defaultHigh),
            message.widthAnchor.constraint(equalTo: stack.widthAnchor),
            detail.widthAnchor.constraint(equalTo: stack.widthAnchor),
            progress.widthAnchor.constraint(equalTo: stack.widthAnchor),
            progress.heightAnchor.constraint(equalToConstant: 6)
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }

    override var isFlipped: Bool { true }

    override func draw(_ dirtyRect: NSRect) {
        NSGradient(starting: NSColor(calibratedRed: 0.055, green: 0.08, blue: 0.105, alpha: 1),
                   ending: NSColor(calibratedRed: 0.014, green: 0.024, blue: 0.04, alpha: 1))?
            .draw(in: bounds, angle: 90)
    }

    func showProgress(_ state: GameImporter.Progress) {
        importing = true
        choose.isEnabled = false
        separate.isEnabled = false
        back.isEnabled = false
        cancel.isHidden = false
        cancel.isEnabled = true
        progress.isHidden = false
        progress.isIndeterminate = state.totalBytes == 0
        if progress.isIndeterminate { progress.startAnimation(nil) }
        else { progress.stopAnimation(nil); progress.doubleValue = state.fraction }
        message.stringValue = state.message
        detail.textColor = NSColor(white: 0.65, alpha: 1)
        detail.stringValue = state.totalBytes == 0 ? "Checking the original files before copying anything."
            : "\(ByteCountFormatter.string(fromByteCount: state.completedBytes, countStyle: .file)) of \(ByteCountFormatter.string(fromByteCount: state.totalBytes, countStyle: .file))"
    }

    func showError(_ error: Error) {
        importing = false
        progress.stopAnimation(nil)
        progress.isHidden = true
        cancel.isHidden = true
        choose.isEnabled = true
        separate.isEnabled = true
        back.isEnabled = true
        message.stringValue = error is GameImporter.ImportError && (error as? GameImporter.ImportError)?.isCancellation == true
            ? "Your import was cancelled." : "The game files could not be imported."
        detail.textColor = NSColor(calibratedRed: 0.91, green: 0.75, blue: 0.57, alpha: 1)
        detail.stringValue = error.localizedDescription
    }

    @objc private func chooseParent() { onChooseParent?() }
    @objc private func chooseSeparately() { onChooseDiscs?() }
    @objc private func cancelImport() { cancel.isEnabled = false; message.stringValue = "Cancelling import…"; onCancel?() }
    @objc private func returnToGame() { onReturn?() }
}

private extension NSLayoutConstraint {
    func withPriority(_ value: NSLayoutConstraint.Priority) -> NSLayoutConstraint { priority = value; return self }
}

private extension GameImporter.ImportError {
    var isCancellation: Bool { if case .cancelled = self { return true }; return false }
}

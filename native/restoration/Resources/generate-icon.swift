#!/usr/bin/swift
// Original Titanic Mac project artwork, authored from geometric primitives.
// No game artwork, photographs, traced images, fonts, or external assets are used.
// Drawing code and resulting artwork are licensed under GPL-3.0.
//
// Reproduce with macOS and Xcode Command Line Tools:
//   swift native/restoration/Resources/generate-icon.swift
// Optional: pass a different output directory as the first argument.
import AppKit
import Foundation

let output = CommandLine.arguments.count > 1
    ? URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
    : URL(fileURLWithPath: #filePath).deletingLastPathComponent()
let manager = FileManager.default
try manager.createDirectory(at: output, withIntermediateDirectories: true)
let temporary = manager.temporaryDirectory.appendingPathComponent("titanic-icon-\(UUID().uuidString)", isDirectory: true)
let iconset = temporary.appendingPathComponent("Titanic.iconset", isDirectory: true)
try manager.createDirectory(at: iconset, withIntermediateDirectories: true)
defer { try? manager.removeItem(at: temporary) }

func color(_ hex: UInt32, _ alpha: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
            green: CGFloat((hex >> 8) & 255) / 255,
            blue: CGFloat(hex & 255) / 255, alpha: alpha)
}

func fill(_ path: NSBezierPath, _ shade: NSColor) {
    shade.setFill()
    path.fill()
}

func stroke(_ path: NSBezierPath, _ shade: NSColor, _ width: CGFloat) {
    shade.setStroke()
    path.lineWidth = width
    path.lineCapStyle = .round
    path.lineJoinStyle = .round
    path.stroke()
}

func line(_ points: [NSPoint], _ shade: NSColor, _ width: CGFloat) {
    let path = NSBezierPath()
    path.move(to: points[0])
    for point in points.dropFirst() { path.line(to: point) }
    stroke(path, shade, width)
}

func draw() {
    let body = NSBezierPath(roundedRect: NSRect(x: 56, y: 56, width: 912, height: 912), xRadius: 198, yRadius: 198)
    NSGraphicsContext.saveGraphicsState()
    let shadow = NSShadow()
    shadow.shadowColor = color(0x000a12, 0.35)
    shadow.shadowBlurRadius = 30
    shadow.shadowOffset = NSSize(width: 0, height: -12)
    shadow.set()
    fill(body, color(0x061621))
    NSGraphicsContext.restoreGraphicsState()
    NSGradient(starting: color(0x17394a), ending: color(0x03111d))!.draw(in: body, angle: 270)
    stroke(body, color(0x648392, 0.34), 3)

    // An original porthole treatment: warm metal around a dark Atlantic sky.
    let outer = NSBezierPath(ovalIn: NSRect(x: 143, y: 143, width: 738, height: 738))
    NSGradient(starting: color(0xf1d593), ending: color(0x856039))!.draw(in: outer, angle: 270)
    let inset = NSBezierPath(ovalIn: NSRect(x: 161, y: 161, width: 702, height: 702))
    NSGradient(starting: color(0x0b2433), ending: color(0x031521))!.draw(in: inset, angle: 270)
    stroke(NSBezierPath(ovalIn: NSRect(x: 168, y: 168, width: 688, height: 688)), color(0xecd299, 0.22), 2)

    NSGraphicsContext.saveGraphicsState()
    inset.addClip()
    // Sky and distant sea, deliberately quiet behind the liner.
    fill(NSBezierPath(rect: NSRect(x: 150, y: 145, width: 730, height: 290)), color(0x123241))
    line([NSPoint(x: 160, y: 435), NSPoint(x: 864, y: 435)], color(0x73929a, 0.22), 3)
    for (x, y, radius, opacity) in [(313.0, 698.0, 3.0, 0.75), (617.0, 755.0, 2.5, 0.7),
                                    (757.0, 673.0, 3.0, 0.6), (466.0, 791.0, 2.0, 0.5)] {
        fill(NSBezierPath(ovalIn: NSRect(x: x-radius, y: y-radius, width: radius*2, height: radius*2)),
             color(0xf1e0b6, opacity))
    }

    // Four broad funnels and a simple stepped deck, drawn without a reference.
    let deck = NSBezierPath()
    deck.move(to: NSPoint(x: 257, y: 459))
    for point in [NSPoint(x: 257, y: 490), NSPoint(x: 304, y: 490), NSPoint(x: 304, y: 517),
                  NSPoint(x: 348, y: 517), NSPoint(x: 348, y: 536), NSPoint(x: 682, y: 536),
                  NSPoint(x: 682, y: 518), NSPoint(x: 730, y: 518), NSPoint(x: 730, y: 490),
                  NSPoint(x: 771, y: 490), NSPoint(x: 771, y: 459)] { deck.line(to: point) }
    deck.close()
    NSGradient(starting: color(0xf8e9c6), ending: color(0xc5af86))!.draw(in: deck, angle: 270)

    for x in [354.0, 443.0, 532.0, 621.0] {
        let funnel = NSBezierPath()
        funnel.move(to: NSPoint(x: x, y: 529))
        funnel.line(to: NSPoint(x: x+47, y: 529))
        funnel.line(to: NSPoint(x: x+37, y: 614))
        funnel.line(to: NSPoint(x: x-10, y: 614))
        funnel.close()
        NSGradient(starting: color(0xf0ba69), ending: color(0xb7793d))!.draw(in: funnel, angle: 0)
        let cap = NSBezierPath()
        cap.move(to: NSPoint(x: x-10, y: 614))
        cap.line(to: NSPoint(x: x+37, y: 614))
        cap.line(to: NSPoint(x: x+35, y: 632))
        cap.line(to: NSPoint(x: x-12, y: 632))
        cap.close()
        fill(cap, color(0x06131b))
    }

    // Masts and taut rigging remain secondary at small icon sizes.
    line([NSPoint(x: 291, y: 487), NSPoint(x: 278, y: 663)], color(0xd9c28f), 7)
    line([NSPoint(x: 742, y: 487), NSPoint(x: 726, y: 661)], color(0xd9c28f), 7)
    line([NSPoint(x: 230, y: 465), NSPoint(x: 278, y: 655), NSPoint(x: 346, y: 531)], color(0xd9c28f, 0.4), 2)
    line([NSPoint(x: 670, y: 531), NSPoint(x: 726, y: 653), NSPoint(x: 806, y: 465)], color(0xd9c28f, 0.4), 2)
    line([NSPoint(x: 279, y: 651), NSPoint(x: 727, y: 649)], color(0xd9c28f, 0.25), 2)

    let hull = NSBezierPath()
    hull.move(to: NSPoint(x: 209, y: 464))
    hull.line(to: NSPoint(x: 829, y: 464))
    hull.curve(to: NSPoint(x: 776, y: 386), controlPoint1: NSPoint(x: 815, y: 423), controlPoint2: NSPoint(x: 806, y: 400))
    hull.line(to: NSPoint(x: 283, y: 386))
    hull.curve(to: NSPoint(x: 209, y: 464), controlPoint1: NSPoint(x: 252, y: 401), controlPoint2: NSPoint(x: 223, y: 432))
    hull.close()
    NSGradient(starting: color(0x233741), ending: color(0x070f18))!.draw(in: hull, angle: 270)
    line([NSPoint(x: 216, y: 462), NSPoint(x: 821, y: 462)], color(0xf0ca83), 7)
    for x in stride(from: 290.0, through: 760.0, by: 30.0) {
        fill(NSBezierPath(ovalIn: NSRect(x: x, y: 430, width: 6, height: 6)), color(0xf5d491, 0.86))
    }
    for x in stride(from: 319.0, through: 707.0, by: 24.0) {
        fill(NSBezierPath(roundedRect: NSRect(x: x, y: 489, width: 9, height: 13), xRadius: 2, yRadius: 2), color(0x15303d))
    }
    // Three clean wave strokes balance the heavy silhouette.
    for (y, width, alpha) in [(354.0, 6.0, 0.65), (316.0, 5.0, 0.45), (279.0, 4.0, 0.3)] {
        let wave = NSBezierPath()
        wave.move(to: NSPoint(x: 266, y: y))
        wave.curve(to: NSPoint(x: 512, y: y), controlPoint1: NSPoint(x: 349, y: y+18), controlPoint2: NSPoint(x: 428, y: y-18))
        wave.curve(to: NSPoint(x: 758, y: y), controlPoint1: NSPoint(x: 595, y: y+18), controlPoint2: NSPoint(x: 679, y: y-18))
        stroke(wave, color(0x8ab2be, alpha), width)
    }
    NSGraphicsContext.restoreGraphicsState()
}

func png(_ pixels: Int) throws -> Data {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels, pixelsHigh: pixels,
                                  bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true,
                                  isPlanar: false, colorSpaceName: .deviceRGB,
                                  bytesPerRow: pixels * 4, bitsPerPixel: 32)!
    let context = NSGraphicsContext(bitmapImageRep: bitmap)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = context
    context.cgContext.clear(CGRect(x: 0, y: 0, width: pixels, height: pixels))
    context.cgContext.scaleBy(x: CGFloat(pixels) / 1024, y: CGFloat(pixels) / 1024)
    draw()
    context.flushGraphics()
    NSGraphicsContext.restoreGraphicsState()
    return bitmap.representation(using: .png, properties: [:])!
}

for size in [16, 32, 128, 256, 512] {
    try png(size).write(to: iconset.appendingPathComponent("icon_\(size)x\(size).png"))
    try png(size * 2).write(to: iconset.appendingPathComponent("icon_\(size)x\(size)@2x.png"))
}
try png(1024).write(to: output.appendingPathComponent("Titanic.png"))
let iconutil = Process()
iconutil.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
iconutil.arguments = ["-c", "icns", iconset.path, "-o", output.appendingPathComponent("Titanic.icns").path]
try iconutil.run()
iconutil.waitUntilExit()
guard iconutil.terminationStatus == 0 else {
    throw NSError(domain: "TitanicIcon", code: Int(iconutil.terminationStatus),
                  userInfo: [NSLocalizedDescriptionKey: "The system icon compiler failed."])
}
print(output.appendingPathComponent("Titanic.icns").path)

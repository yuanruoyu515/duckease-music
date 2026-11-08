#!/usr/bin/env swift
import AppKit
import CoreText

// Rasterize the README banners without shipping font files.
let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let fontDirectory = URL(fileURLWithPath: "/Applications/Microsoft Excel.app/Contents/Resources/DFonts")
for file in ["Century Gothic.ttf", "Century Gothic Bold.ttf"] {
    let url = fontDirectory.appendingPathComponent(file)
    if FileManager.default.fileExists(atPath: url.path) {
        CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
    }
}
func rgb(_ hex: UInt32, alpha: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
            green: CGFloat((hex >> 8) & 255) / 255,
            blue: CGFloat(hex & 255) / 255, alpha: alpha)
}
func text(_ value: String, x: CGFloat, y: CGFloat, size: CGFloat, color: NSColor, bold: Bool = false, tracking: CGFloat = 0) {
    let font = NSFont(name: bold ? "CenturyGothic-Bold" : "CenturyGothic", size: size)
        ?? NSFont.systemFont(ofSize: size, weight: bold ? .bold : .regular)
    (value as NSString).draw(at: NSPoint(x: x, y: y), withAttributes: [
        .font: font, .foregroundColor: color, .kern: tracking,
    ])
}
func roundRect(_ rect: NSRect, radius: CGFloat, color: NSColor) {
    color.setFill()
    NSBezierPath(roundedRect: rect, xRadius: radius, yRadius: radius).fill()
}

for dark in [false, true] {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1600, pixelsHigh: 560,
                                  bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true,
                                  isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    let foreground = rgb(dark ? 0xFAF4ED : 0x272326)
    let muted = rgb(dark ? 0xC5B4AA : 0x70645E)
    let red = rgb(dark ? 0xF37B6F : 0xC63E34)
    // No background fill: the banner sits on the page's own color.

    // Record grooves continue beyond the frame, behind the duck.
    for radius in stride(from: CGFloat(220), through: 540, by: 22) {
        let path = NSBezierPath(ovalIn: NSRect(x: 1285-radius, y: 275-radius, width: radius*2, height: radius*2))
        rgb(dark ? 0xD49B7B : 0xC99175, alpha: 0.10).setStroke()
        path.lineWidth = 1
        path.stroke()
    }
    roundRect(NSRect(x: 74, y: 442, width: 286, height: 34), radius: 17,
              color: rgb(dark ? 0x5E302B : 0xF2DDD4))
    text("PERSONAL CONTINUATION", x: 93, y: 452, size: 12, color: red, bold: true, tracking: 1.15)
    text("DuckEase", x: 70, y: 310, size: 91, color: foreground, bold: true, tracking: -4)
    text("Music", x: 70, y: 218, size: 91, color: red, bold: true, tracking: -4)
    text("A little room for music.", x: 76, y: 157, size: 26, color: muted)

    rgb(dark ? 0x51443C : 0xD7C7B8).setFill()
    NSRect(x: 76, y: 102, width: 570, height: 1).fill()
    text("BASED ON MYGO MUSIC", x: 76, y: 67, size: 12, color: muted, bold: true, tracking: 1.3)
    text("MAINTAINED BY RUOYU", x: 365, y: 67, size: 12, color: muted, bold: true, tracking: 1.3)

    let icon = NSImage(contentsOf: root.appendingPathComponent("resources/icon.png"))!
    let shadow = NSShadow()
    shadow.shadowColor = rgb(0x32190F, alpha: dark ? 0.35 : 0.14)
    shadow.shadowBlurRadius = 38
    shadow.shadowOffset = NSSize(width: 0, height: -14)
    NSGraphicsContext.saveGraphicsState()
    shadow.set()
    icon.draw(in: NSRect(x: 1032, y: 73, width: 418, height: 418))
    NSGraphicsContext.restoreGraphicsState()
    roundRect(NSRect(x: 1101, y: 44, width: 280, height: 36), radius: 18,
              color: rgb(dark ? 0x392A25 : 0xEEE3D7))
    text("NATIVE GO  /  INDEPENDENT FORK", x: 1121, y: 57, size: 10, color: muted, bold: true, tracking: 0.6)
    NSGraphicsContext.restoreGraphicsState()
    let output = root.appendingPathComponent("assets/brand/banner-\(dark ? "dark" : "light").png")
    try FileManager.default.createDirectory(at: output.deletingLastPathComponent(), withIntermediateDirectories: true)
    try bitmap.representation(using: .png, properties: [:])!.write(to: output)
    print(output.lastPathComponent)
}

#!/usr/bin/env swift
import AppKit
import CoreText

// Rasterize the README banners (English and Simplified Chinese, light and
// dark) on a transparent canvas, without shipping font files.
// The square app icon comes from assets/brand/app-icon.png.
let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let fontDirectory = URL(fileURLWithPath: "/Applications/Microsoft Excel.app/Contents/Resources/DFonts")
for file in ["Century Gothic.ttf", "Century Gothic Bold.ttf"] {
    let url = fontDirectory.appendingPathComponent(file)
    if FileManager.default.fileExists(atPath: url.path) {
        CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
    }
}

func rgb(_ hex: UInt32, _ alpha: CGFloat = 1) -> NSColor {
    NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
            green: CGFloat((hex >> 8) & 255) / 255,
            blue: CGFloat(hex & 255) / 255, alpha: alpha)
}

// Century Gothic for Latin text, PingFang SC for Chinese.
func font(_ size: CGFloat, bold: Bool = false, chinese: Bool = false) -> NSFont {
    let name = chinese ? (bold ? "PingFangSC-Semibold" : "PingFangSC-Regular")
                       : (bold ? "CenturyGothic-Bold" : "CenturyGothic")
    return NSFont(name: name, size: size) ?? NSFont.systemFont(ofSize: size, weight: bold ? .bold : .regular)
}

// Draws text with its baseline at y (flipped canvas: y grows downward).
@discardableResult
func text(_ value: String, x: CGFloat, y: CGFloat, font: NSFont, color: NSColor, tracking: CGFloat = 0) -> CGFloat {
    let s = NSAttributedString(string: value, attributes: [.font: font, .foregroundColor: color, .kern: tracking])
    s.draw(at: NSPoint(x: x, y: y - font.ascender))
    return s.size().width
}

func roundRect(_ r: NSRect, _ radius: CGFloat, fill: NSColor, stroke: NSColor? = nil) {
    let p = NSBezierPath(roundedRect: r, xRadius: radius, yRadius: radius)
    fill.setFill(); p.fill()
    if let stroke { stroke.setStroke(); p.lineWidth = 1; p.stroke() }
}

struct Copy {
    let code: String
    let eyebrow: String
    let eyebrowChinese: Bool
    let lines: [(String, Bool)] // (text, chinese)
    let lyricLabel: String
}

let copies = [
    Copy(code: "en",
         eyebrow: "Unofficial desktop music client · native Go, no WebView",
         eyebrowChinese: false,
         lines: [("A little room for music.", false), ("给音乐留一点空间。", true)],
         lyricLabel: "LYRICS"),
    Copy(code: "zh-Hans",
         eyebrow: "非官方桌面音乐客户端 · Go 原生绘制，不用 WebView",
         eyebrowChinese: true,
         lines: [("给音乐留一点空间。", true), ("A little room for music.", false)],
         lyricLabel: "歌词"),
]

let icon = NSImage(contentsOf: root.appendingPathComponent("assets/brand/app-icon.png"))!
let W: CGFloat = 1280, H: CGFloat = 420, scale: CGFloat = 2

for copy in copies {
    for dark in [false, true] {
        let ink = rgb(dark ? 0xF2F3F5 : 0x15171C)
        let muted = rgb(dark ? 0xA3A9B4 : 0x646D7E)
        let faint = rgb(dark ? 0x6B7280 : 0xA7AEBB)
        let red = rgb(dark ? 0xF0675B : 0xD63A32)
        let cardFill = rgb(dark ? 0x171A20 : 0xFFFFFF)
        let cardLine = rgb(dark ? 0x2A2F38 : 0xDFE4ED)
        let track = rgb(dark ? 0x2A2F38 : 0xE8EBF1)

        let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(W * scale), pixelsHigh: Int(H * scale),
                                      bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                      colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
        bitmap.size = NSSize(width: W, height: H)
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap, )
        let ctx = NSGraphicsContext(bitmapImageRep: bitmap)!
        NSGraphicsContext.current = NSGraphicsContext(cgContext: ctx.cgContext, flipped: true)
        ctx.cgContext.translateBy(x: 0, y: H)
        ctx.cgContext.scaleBy(x: 1, y: -1)

        // Eyebrow.
        roundRect(NSRect(x: 56, y: 50, width: 9, height: 9), 2.5, fill: red)
        text(copy.eyebrow, x: 76, y: 59, font: font(16, chinese: copy.eyebrowChinese), color: muted)

        // Title.
        let titleFont = font(76, bold: true)
        let w = text("DuckEase ", x: 52, y: 146, font: titleFont, color: ink, tracking: -2.5)
        text("Music", x: 52 + w, y: 146, font: titleFont, color: red, tracking: -2.5)

        // Tagline, two languages.
        var y: CGFloat = 196
        for (i, line) in copy.lines.enumerated() {
            text(line.0, x: 56, y: y, font: font(i == 0 ? 26 : 21, chinese: line.1), color: i == 0 ? ink.withAlphaComponent(0.82) : muted)
            y += 36
        }

        // Now-playing card, with the project's own demo track and lyric.
        let card = NSRect(x: 56, y: 296, width: 1168, height: 84)
        NSGraphicsContext.saveGraphicsState()
        let shadow = NSShadow()
        shadow.shadowColor = rgb(0x0B1220, dark ? 0.45 : 0.08)
        shadow.shadowBlurRadius = 24
        shadow.shadowOffset = NSSize(width: 0, height: -6)
        shadow.set()
        roundRect(card, 20, fill: cardFill)
        NSGraphicsContext.restoreGraphicsState()
        roundRect(card, 20, fill: .clear, stroke: cardLine)

        // Sleeve: a small gradient square with a record.
        let sleeve = NSRect(x: card.minX + 20, y: card.minY + 16, width: 52, height: 52)
        let sleevePath = NSBezierPath(roundedRect: sleeve, xRadius: 10, yRadius: 10)
        NSGradient(starting: rgb(0xF2A33A), ending: rgb(0xD63A32))!.draw(in: sleevePath, angle: -45)
        rgb(0xFFFFFF, 0.9).setStroke()
        let disc = NSBezierPath(ovalIn: sleeve.insetBy(dx: 14, dy: 14)); disc.lineWidth = 4; disc.stroke()

        text("给未来的自己", x: sleeve.maxX + 16, y: card.minY + 37, font: font(19, bold: true, chinese: true), color: ink)
        text("DuckEase Demo · 给自己一点时间", x: sleeve.maxX + 16, y: card.minY + 62, font: font(14, chinese: true), color: muted)

        // Divider, then the current lyric line.
        rgb(dark ? 0x2A2F38 : 0xE8EBF1).setFill()
        NSRect(x: card.minX + 360, y: card.minY + 20, width: 1, height: 44).fill()
        text(copy.lyricLabel, x: card.minX + 388, y: card.minY + 34, font: font(11, bold: true, chinese: copy.code != "en"), color: faint, tracking: copy.code == "en" ? 1.4 : 0.5)
        text("我要握住 一个最美的梦 给未来的自己", x: card.minX + 388, y: card.minY + 62, font: font(21, bold: true, chinese: true), color: red)

        // Progress and controls.
        let barX = card.minX + 790, barW: CGFloat = 240, barY = card.minY + 42
        roundRect(NSRect(x: barX, y: barY - 2, width: barW, height: 4), 2, fill: track)
        roundRect(NSRect(x: barX, y: barY - 2, width: barW * 0.33, height: 4), 2, fill: red)
        NSBezierPath(ovalIn: NSRect(x: barX + barW * 0.33 - 6, y: barY - 6, width: 12, height: 12)).fill()
        text("1:12", x: barX, y: barY + 26, font: font(12), color: faint)
        let total = "3:38"
        let tw = (total as NSString).size(withAttributes: [.font: font(12)]).width
        text(total, x: barX + barW - tw, y: barY + 26, font: font(12), color: faint)

        // Play button.
        let play = NSRect(x: card.maxX - 72, y: card.minY + 20, width: 44, height: 44)
        red.setFill(); NSBezierPath(ovalIn: play).fill()
        cardFill.setFill()
        NSRect(x: play.midX - 7, y: play.midY - 8, width: 4.5, height: 16).fill()
        NSRect(x: play.midX + 2.5, y: play.midY - 8, width: 4.5, height: 16).fill()

        // Square app icon, slightly tilted, with a soft shadow.
        NSGraphicsContext.saveGraphicsState()
        let iconShadow = NSShadow()
        iconShadow.shadowColor = rgb(0x0B1220, dark ? 0.5 : 0.14)
        iconShadow.shadowBlurRadius = 30
        iconShadow.shadowOffset = NSSize(width: 0, height: -12)
        iconShadow.set()
        let side: CGFloat = 268
        let center = NSPoint(x: 1078, y: 140)
        let t = NSAffineTransform()
        t.translateX(by: center.x, yBy: center.y)
        t.rotate(byDegrees: 4)
        t.concat()
        icon.draw(in: NSRect(x: -side / 2, y: -side / 2, width: side, height: side),
                  from: .zero, operation: .sourceOver, fraction: 1, respectFlipped: true, hints: nil)
        NSGraphicsContext.restoreGraphicsState()

        NSGraphicsContext.restoreGraphicsState()
        let output = root.appendingPathComponent("assets/brand/banner-\(copy.code)-\(dark ? "dark" : "light").png")
        try bitmap.representation(using: .png, properties: [:])!.write(to: output)
        print(output.lastPathComponent)
    }
}

#!/usr/bin/env swift
// Export the square app icon, as macOS renders it from a local build, to a 1024 px PNG.
// Usage: swift scripts/export-app-icon.swift "$PWD/build/darwin-universal/DuckEase Music.app" assets/brand/app-icon.png
import AppKit
let img = NSWorkspace.shared.icon(forFile: CommandLine.arguments[1])
let side = 1024
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
img.draw(in: NSRect(x: 0, y: 0, width: side, height: side), from: .zero, operation: .copy, fraction: 1)
NSGraphicsContext.restoreGraphicsState()
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[2]))

// Rasterizes SF Symbols to PNG. Reads one name per line from stdin; writes a
// stream of [uint32 big-endian length][PNG bytes] per name, in order. A
// missing symbol gets length 0. Args: <pointSize> <hexColor rrggbb>.
import AppKit

let args = CommandLine.arguments
let pt = args.count > 1 ? CGFloat(Double(args[1]) ?? 48) : 48
let hex = args.count > 2 ? args[2] : "ffffff"
let weightName = args.count > 3 ? args[3] : "semibold"

func weight(_ s: String) -> NSFont.Weight {
    switch s {
    case "ultralight": return .ultraLight
    case "thin": return .thin
    case "light": return .light
    case "regular": return .regular
    case "medium": return .medium
    case "semibold": return .semibold
    case "bold": return .bold
    case "heavy": return .heavy
    case "black": return .black
    default: return .semibold
    }
}

func tint(_ h: String) -> NSColor {
    var v: UInt64 = 0
    Scanner(string: h).scanHexInt64(&v)
    return NSColor(srgbRed: CGFloat((v >> 16) & 0xff) / 255,
                   green: CGFloat((v >> 8) & 0xff) / 255,
                   blue: CGFloat(v & 0xff) / 255, alpha: 1)
}
let color = tint(hex)
let out = FileHandle.standardOutput

func emit(_ data: Data?) {
    var n = UInt32(data?.count ?? 0).bigEndian
    out.write(Data(bytes: &n, count: 4))
    if let d = data { out.write(d) }
}

func render(_ name: String) -> Data? {
    guard let base = NSImage(systemSymbolName: name, accessibilityDescription: nil) else { return nil }
    let cfg = NSImage.SymbolConfiguration(pointSize: pt, weight: weight(weightName))
    let img = base.withSymbolConfiguration(cfg) ?? base
    let sz = img.size
    guard sz.width > 0, sz.height > 0,
          let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(sz.width),
                                     pixelsHigh: Int(sz.height), bitsPerSample: 8, samplesPerPixel: 4,
                                     hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                                     bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    color.set()
    let r = NSRect(origin: .zero, size: sz)
    img.draw(in: r)
    r.fill(using: .sourceAtop) // tint the template glyph
    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])
}

while let line = readLine(strippingNewline: true) {
    emit(render(line))
}

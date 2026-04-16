// go run ./cmd/genicon  — generates icons from favicon.svg geometry.
// Writes:
//   internal/tray/icon_windows.go  — BMP-based ICO bytes (LoadImage compatible)
//   internal/tray/icon_unix.go     — PNG bytes (macOS / Linux)
//   build/appicon.png              — 256×256 PNG (Wails window icon)
//   build/windows/icon.ico         — PNG-inside-ICO (embedded in .exe by Wails linker)
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// ── colour helpers ────────────────────────────────────────────────────────────

func lerp8(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t)
}

func blendOver(dst, fg color.RGBA) color.RGBA {
	if fg.A == 0 {
		return dst
	}
	if fg.A == 255 {
		return fg
	}
	a := float64(fg.A) / 255.0
	return color.RGBA{lerp8(dst.R, fg.R, a), lerp8(dst.G, fg.G, a), lerp8(dst.B, fg.B, a), 255}
}

// ── geometry drawing ──────────────────────────────────────────────────────────

// fillRoundRect draws a filled rounded rectangle onto img.
// Geometry is in SVG-space units (32×32); scale maps to pixel-space.
func fillRoundRect(img *image.RGBA, x, y, w, h, rx float64, c color.RGBA, scale float64) {
	x0 := int(math.Round(x * scale))
	y0 := int(math.Round(y * scale))
	x1 := int(math.Round((x + w) * scale))
	y1 := int(math.Round((y + h) * scale))
	rxs := rx * scale

	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			fpx, fpy := float64(px)+0.5, float64(py)+0.5
			fx0, fy0 := float64(x0), float64(y0)
			fx1, fy1 := float64(x1), float64(y1)
			var cx, cy float64
			inCorner := true
			switch {
			case fpx < fx0+rxs && fpy < fy0+rxs:
				cx, cy = fx0+rxs, fy0+rxs
			case fpx >= fx1-rxs && fpy < fy0+rxs:
				cx, cy = fx1-rxs, fy0+rxs
			case fpx < fx0+rxs && fpy >= fy1-rxs:
				cx, cy = fx0+rxs, fy1-rxs
			case fpx >= fx1-rxs && fpy >= fy1-rxs:
				cx, cy = fx1-rxs, fy1-rxs
			default:
				inCorner = false
			}
			if inCorner {
				dx, dy := fpx-cx, fpy-cy
				if math.Sqrt(dx*dx+dy*dy) > rxs {
					continue
				}
			}
			img.SetRGBA(px, py, blendOver(img.RGBAAt(px, py), c))
		}
	}
}

// ── render favicon geometry ───────────────────────────────────────────────────

func renderIcon(size int) *image.RGBA {
	scale := float64(size) / 32.0
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// transparent fill
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{})
		}
	}

	bg := color.RGBA{0x11, 0x11, 0x11, 0xFF}
	blue := color.RGBA{0x4a, 0x9e, 0xff, 0xFF}
	b90 := color.RGBA{blue.R, blue.G, blue.B, uint8(math.Round(0.9 * 255))}
	b55 := color.RGBA{blue.R, blue.G, blue.B, uint8(math.Round(0.55 * 255))}

	// <rect width="32" height="32" rx="6" fill="#111111"/>
	fillRoundRect(img, 0, 0, 32, 32, 6, bg, scale)
	// <rect x="2"  y="2"  width="13" height="10" rx="2" fill="#4a9eff" opacity="0.9"/>
	fillRoundRect(img, 2, 2, 13, 10, 2, b90, scale)
	// <rect x="17" y="2"  width="13" height="10" rx="2" fill="#4a9eff" opacity="0.55"/>
	fillRoundRect(img, 17, 2, 13, 10, 2, b55, scale)
	// <rect x="2"  y="14" width="13" height="16" rx="2" fill="#4a9eff" opacity="0.55"/>
	fillRoundRect(img, 2, 14, 13, 16, 2, b55, scale)
	// <rect x="17" y="14" width="13" height="16" rx="2" fill="#4a9eff" opacity="0.9"/>
	fillRoundRect(img, 17, 14, 13, 16, 2, b90, scale)

	return img
}

func encodePNG(img *image.RGBA) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// ── BMP-inside-ICO encoder ────────────────────────────────────────────────────
// LoadImage(IMAGE_ICON, LR_LOADFROMFILE) only accepts traditional BMP-ICO,
// not the PNG-inside-ICO variant introduced in Vista.

func bmpIcoEntry(img *image.RGBA) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	var buf bytes.Buffer

	// BITMAPINFOHEADER (40 bytes)
	binary.Write(&buf, binary.LittleEndian, uint32(40))     // biSize
	binary.Write(&buf, binary.LittleEndian, int32(w))       // biWidth
	binary.Write(&buf, binary.LittleEndian, int32(h*2))     // biHeight (ICO: doubled)
	binary.Write(&buf, binary.LittleEndian, uint16(1))      // biPlanes
	binary.Write(&buf, binary.LittleEndian, uint16(32))     // biBitCount
	binary.Write(&buf, binary.LittleEndian, uint32(0))      // biCompression (BI_RGB)
	binary.Write(&buf, binary.LittleEndian, uint32(0))      // biSizeImage
	binary.Write(&buf, binary.LittleEndian, int32(0))       // biXPelsPerMeter
	binary.Write(&buf, binary.LittleEndian, int32(0))       // biYPelsPerMeter
	binary.Write(&buf, binary.LittleEndian, uint32(0))      // biClrUsed
	binary.Write(&buf, binary.LittleEndian, uint32(0))      // biClrImportant

	// XOR mask: BGRA, bottom-up (row h-1 first)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

	// AND mask: 1-bit per pixel, 0=visible, bottom-up, rows padded to 4 bytes.
	// All zero → let 32-bit alpha channel handle transparency (Vista+ behaviour).
	andRowBytes := (w + 31) / 32 * 4
	zeros := make([]byte, andRowBytes)
	for y := 0; y < h; y++ {
		buf.Write(zeros)
	}

	return buf.Bytes()
}

// generateBmpICO builds an ICO file with traditional BMP entries for each size.
func generateBmpICO(sizes []int) []byte {
	type entry struct {
		sz   int
		data []byte
	}
	var entries []entry
	for _, sz := range sizes {
		entries = append(entries, entry{sz, bmpIcoEntry(renderIcon(sz))})
	}

	// ICONDIR header + ICONDIRENTRY array
	const icoHdr = 6
	const icoEntry = 16
	dataOffset := uint32(icoHdr + icoEntry*len(entries))

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(0))             // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))             // type = ICO
	binary.Write(&buf, binary.LittleEndian, uint16(len(entries)))  // count

	offset := dataOffset
	for _, e := range entries {
		sz := uint8(e.sz) // uint8(256) wraps to 0, which is ICO convention for 256
		binary.Write(&buf, binary.LittleEndian, sz)                // bWidth
		binary.Write(&buf, binary.LittleEndian, sz)                // bHeight
		binary.Write(&buf, binary.LittleEndian, uint8(0))          // bColorCount
		binary.Write(&buf, binary.LittleEndian, uint8(0))          // bReserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))         // wPlanes
		binary.Write(&buf, binary.LittleEndian, uint16(32))        // wBitCount
		binary.Write(&buf, binary.LittleEndian, uint32(len(e.data)))
		binary.Write(&buf, binary.LittleEndian, offset)
		offset += uint32(len(e.data))
	}
	for _, e := range entries {
		buf.Write(e.data)
	}
	return buf.Bytes()
}

// generatePngICO builds an ICO file with PNG entries (Vista+).
// Used for the .exe resource (embedded by Wails build); NOT for LoadImage().
func generatePngICO(sizes []int) []byte {
	var pngs [][]byte
	for _, sz := range sizes {
		pngs = append(pngs, encodePNG(renderIcon(sz)))
	}

	const icoHdr = 6
	const icoEntry = 16
	dataOffset := uint32(icoHdr + icoEntry*len(sizes))

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(len(sizes)))

	offset := dataOffset
	for i, sz := range sizes {
		sz8 := uint8(sz)
		binary.Write(&buf, binary.LittleEndian, sz8)
		binary.Write(&buf, binary.LittleEndian, sz8)
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint16(1))
		binary.Write(&buf, binary.LittleEndian, uint16(32))
		binary.Write(&buf, binary.LittleEndian, uint32(len(pngs[i])))
		binary.Write(&buf, binary.LittleEndian, offset)
		offset += uint32(len(pngs[i]))
	}
	for _, p := range pngs {
		buf.Write(p)
	}
	return buf.Bytes()
}

// ── Go source writers ─────────────────────────────────────────────────────────

func writeGoIcon(path, buildTag, varComment string, data []byte) error {
	var sb bytes.Buffer
	if buildTag != "" {
		fmt.Fprintf(&sb, "//go:build %s\n\n", buildTag)
	}
	sb.WriteString("package tray\n\n")
	fmt.Fprintf(&sb, "// %s\n", varComment)
	sb.WriteString("var iconPNG = []byte{")
	for i, b := range data {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "0x%02x", b)
	}
	sb.WriteString("}\n")
	return os.WriteFile(path, sb.Bytes(), 0644)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	// Remove legacy icon.go (no build tag) if it exists
	_ = os.Remove(root + "/internal/tray/icon.go")

	// Windows tray icon: BMP-inside-ICO (works with LoadImage IMAGE_ICON)
	bmpIco := generateBmpICO([]int{16, 32, 48})
	winPath := root + "/internal/tray/icon_windows.go"
	if err := writeGoIcon(winPath, "windows",
		"iconPNG holds BMP-based ICO bytes (16/32/48 px). LoadImage(IMAGE_ICON) requires ICO, not PNG.",
		bmpIco); err != nil {
		fmt.Println("icon_windows.go:", err)
	} else {
		fmt.Printf("wrote %s  (%d bytes of ICO)\n", winPath, len(bmpIco))
	}

	// macOS / Linux tray icon: PNG
	png32 := encodePNG(renderIcon(32))
	unixPath := root + "/internal/tray/icon_unix.go"
	if err := writeGoIcon(unixPath, "!windows",
		"iconPNG holds a 32×32 PNG icon for macOS/Linux systray.",
		png32); err != nil {
		fmt.Println("icon_unix.go:", err)
	} else {
		fmt.Printf("wrote %s  (%d bytes of PNG)\n", unixPath, len(png32))
	}

	// Wails app icon: 256×256 PNG
	png256 := encodePNG(renderIcon(256))
	if err := os.WriteFile(root+"/build/appicon.png", png256, 0644); err != nil {
		fmt.Println("appicon.png:", err)
	} else {
		fmt.Printf("wrote build/appicon.png  (%d bytes)\n", len(png256))
	}

	// Windows .exe icon: PNG-inside-ICO (Wails linker embeds it as resource)
	pngIco := generatePngICO([]int{16, 32, 48, 256})
	if err := os.WriteFile(root+"/build/windows/icon.ico", pngIco, 0644); err != nil {
		fmt.Println("icon.ico:", err)
	} else {
		fmt.Printf("wrote build/windows/icon.ico  (%d bytes)\n", len(pngIco))
	}

	fmt.Println("done.")
}

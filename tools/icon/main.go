// Command icon draws the devstack icons: a Terracotta tile with three stacked
// bars, plus a variant with a green "running" dot for the tray.
//
//	go run ./tools/icon
//
// Writes cmd/devstack-app/winres/icon.png (exe icon, 256px) and
// cmd/devstack-app/icons/{tray,tray-running}.ico (16-48px).
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

var (
	terracotta = color.NRGBA{0xA8, 0x47, 0x26, 0xFF}
	paper      = color.NRGBA{0xFF, 0xFD, 0xFA, 0xFF}
	live       = color.NRGBA{0x2E, 0x8A, 0x52, 0xFF}
)

func main() {
	log.SetFlags(0)
	app := "cmd/devstack-app"
	must(os.MkdirAll(filepath.Join(app, "winres"), 0o755))
	must(os.MkdirAll(filepath.Join(app, "icons"), 0o755))
	must(writePNG(filepath.Join(app, "winres", "icon.png"), draw(256, false)))
	must(writeICO(filepath.Join(app, "icons", "tray.ico"), false, 16, 20, 24, 32, 40, 48))
	must(writeICO(filepath.Join(app, "icons", "tray-running.ico"), true, 16, 20, 24, 32, 40, 48))
}

// draw renders the icon at size px with 4x4 supersampling.
func draw(size int, running bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	s := float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/ss) / s
					py := (float64(y) + (float64(sy)+0.5)/ss) / s
					c, ok := sample(px, py, running)
					if !ok {
						continue
					}
					af := float64(c.A) / 255
					r += float64(c.R) * af
					g += float64(c.G) * af
					b += float64(c.B) * af
					a += af
				}
			}
			n := float64(ss * ss)
			if a == 0 {
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / a), G: uint8(g / a), B: uint8(b / a), A: uint8(math.Round(a / n * 255)),
			})
		}
	}
	return img
}

// sample returns the colour at (x, y) in unit coordinates, top-most shape first.
func sample(x, y float64, running bool) (color.NRGBA, bool) {
	if running {
		cx, cy, rad := 0.79, 0.79, 0.2
		d := math.Hypot(x-cx, y-cy)
		if d <= rad {
			return live, true
		}
		if d <= rad+0.06 {
			return color.NRGBA{}, false // transparent ring separates the dot from the tile
		}
	}
	// Three server slabs, each with an indicator light cut out on the left.
	for _, y0 := range []float64{0.21, 0.425, 0.64} {
		y1 := y0 + 0.155
		if roundRect(x, y, 0.21, y0, 0.79, y1, 0.045) {
			if math.Hypot(x-0.32, y-(y0+y1)/2) <= 0.032 {
				return terracotta, true
			}
			return paper, true
		}
	}
	if roundRect(x, y, 0.04, 0.04, 0.96, 0.96, 0.2) {
		return terracotta, true
	}
	return color.NRGBA{}, false
}

func roundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return math.Hypot(x-cx, y-cy) <= r
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writeICO packs PNG-compressed images (supported since Windows Vista).
func writeICO(path string, running bool, sizes ...int) error {
	var pngs [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, draw(s, running)); err != nil {
			return err
		}
		pngs = append(pngs, buf.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0
		}
		binary.Write(&out, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(pngs[i])), uint32(offset)})
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

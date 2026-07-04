//go:build !cli

package apptray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

var (
	dotGreen []byte // online — RGB(0, 210, 90)
	dotRed   []byte // offline — RGB(255, 59, 48)
)

func init() {
	dotGreen = makeDot(0, 210, 90)
	dotRed = makeDot(255, 59, 48)
}

// makeDot generates a 22×22 PNG with a play-button icon (circle + triangle)
// in the given color on a transparent background.
func makeDot(r, g, b uint8) []byte {
	const size = 22
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	cx, cy := float64(size)/2, float64(size)/2
	circleR := float64(size)/2 - 1.5

	ts := circleR * 0.52
	ox := ts * 0.08
	p1x, p1y := cx-ts*0.62+ox, cy-ts
	p2x, p2y := cx-ts*0.62+ox, cy+ts
	p3x, p3y := cx+ts*0.82+ox, cy

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			dx, dy := fx-cx, fy-cy
			dist := math.Sqrt(dx*dx + dy*dy)

			alpha := math.Min(1.0, math.Max(0.0, circleR+0.5-dist))
			if alpha <= 0 {
				continue
			}
			a := uint8(alpha * 255)
			if inPlayTriangle(fx, fy, p1x, p1y, p2x, p2y, p3x, p3y) {
				img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: a})
			} else {
				img.Set(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func inPlayTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (px-bx)*(ay-by) - (ax-bx)*(py-by)
	d2 := (px-cx)*(by-cy) - (bx-cx)*(py-cy)
	d3 := (px-ax)*(cy-ay) - (cx-ax)*(py-ay)
	hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0)
	hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0)
	return !(hasNeg && hasPos)
}

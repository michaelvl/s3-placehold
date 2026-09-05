package image

import (
	stdimage "image"
	"image/color"
	"math"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// Guide geometry, all as fractions of the image's shorter side so an overlay
// looks the same on a thumbnail and on a poster, then clamped so it stays
// legible at either extreme.
const (
	guideStrokeRatio = 0.004 // line thickness
	guideStrokeMin   = 1
	guideStrokeMax   = 6

	guideArrowRatio     = 0.06 // arrowhead length, tip to base
	guideArrowMin       = 4.0
	guideArrowMax       = 48.0
	guideArrowWidthNorm = 0.8 // base width, as a fraction of the length

	guideCornerRatio = 0.12 // crop-mark arm length
	guideCornerMin   = 6
	guideCornerMax   = 96
	// Crop marks are drawn heavier than the other guides. Flush with the edge
	// at the common stroke they would be a pixel-subset of `frame`, so
	// `guides=corners,frame` would render as plain `guides=frame`; the extra
	// weight is also how a crop mark looks on a real print reference.
	guideCornerStrokeScale = 3
)

// guideRect is an axis-aligned filled band, in whole pixels. Guides are
// described as rects rather than stroked lines so that the SVG and raster
// renderers cannot disagree about which pixels a line covers: an integer rect
// lands on pixel boundaries, so an SVG renderer has nothing to antialias.
type guideRect struct {
	x, y, w, h int
}

// guideTri is a filled triangle — the arrowheads, the one guide shape that is
// not axis-aligned. Its diagonal edges do antialias in SVG and do not in the
// raster output, so the two agree in shape but not to the pixel along those
// two edges.
type guideTri struct {
	pts [3][2]float64
}

// guideSpec is the resolved, pixel-space description of the guide overlay,
// computed once by guideGeometry and consumed twice: renderSVG formats it into
// markup and renderRaster fills it. Same arrangement, and same reason, as
// gradientSpec.
type guideSpec struct {
	rects []guideRect
	tris  []guideTri
}

// empty reports whether the spec draws nothing.
func (s guideSpec) empty() bool { return len(s.rects) == 0 && len(s.tris) == 0 }

// guideGeometry resolves guides into shapes covering a w-by-h image. Every
// shape lies inside the image bounds — the point of the overlay is that
// cropping the result removes part of it.
func guideGeometry(w, h int, guides []key.Guide) guideSpec {
	var spec guideSpec
	if len(guides) == 0 || w <= 0 || h <= 0 {
		return spec
	}

	short := math.Min(float64(w), float64(h))
	stroke := clampInt(int(math.Round(short*guideStrokeRatio)), guideStrokeMin, guideStrokeMax)

	for _, g := range guides {
		switch g {
		case key.GuideThirds:
			spec.addThirds(w, h, stroke)
		case key.GuideFrame:
			spec.addFrame(w, h, stroke)
		case key.GuideCorners:
			arm := clampInt(int(math.Round(short*guideCornerRatio)), guideCornerMin, guideCornerMax)
			// The clamped minimums are what can overrun a very small image, so
			// both are capped against the image itself; addCorners caps the arm.
			spec.addCorners(w, h, clampInt(stroke*guideCornerStrokeScale, 1, int(short)), arm)
		case key.GuideCross:
			arrow := clampFloat(short*guideArrowRatio, guideArrowMin, guideArrowMax)
			// Ditto: an arrowhead longer than half the shorter side would reach
			// past the border its opposite arrowhead points at.
			spec.addCross(w, h, stroke, math.Min(arrow, short/2))
		}
	}
	return spec
}

// addThirds draws the rule-of-thirds grid: the image split in three along each
// axis.
func (s *guideSpec) addThirds(w, h, stroke int) {
	for i := 1; i <= 2; i++ {
		s.rects = append(s.rects,
			vBand(float64(w)*float64(i)/3, h, stroke),
			hBand(float64(h)*float64(i)/3, w, stroke),
		)
	}
}

// addFrame outlines the image along its very edge, so that cropping any side
// removes that side's line entirely.
func (s *guideSpec) addFrame(w, h, stroke int) {
	s.rects = append(s.rects,
		guideRect{x: 0, y: 0, w: w, h: stroke},
		guideRect{x: 0, y: h - stroke, w: w, h: stroke},
		guideRect{x: 0, y: 0, w: stroke, h: h},
		guideRect{x: w - stroke, y: 0, w: stroke, h: h},
	)
}

// addCorners draws an L-shaped crop mark flush with each corner. Print crop
// marks sit outside the trim; these sit inside it, because anything outside
// the image is not in the image.
func (s *guideSpec) addCorners(w, h, stroke, arm int) {
	if arm > w {
		arm = w
	}
	if arm > h {
		arm = h
	}
	s.rects = append(s.rects,
		// Top-left.
		guideRect{x: 0, y: 0, w: arm, h: stroke},
		guideRect{x: 0, y: 0, w: stroke, h: arm},
		// Top-right.
		guideRect{x: w - arm, y: 0, w: arm, h: stroke},
		guideRect{x: w - stroke, y: 0, w: stroke, h: arm},
		// Bottom-left.
		guideRect{x: 0, y: h - stroke, w: arm, h: stroke},
		guideRect{x: 0, y: h - arm, w: stroke, h: arm},
		// Bottom-right.
		guideRect{x: w - arm, y: h - stroke, w: arm, h: stroke},
		guideRect{x: w - stroke, y: h - arm, w: stroke, h: arm},
	)
}

// addCross draws the centre crosshair: a full-width and a full-height line
// meeting at the centre, each ending in an arrowhead whose tip touches the
// border. The arrowheads are what make a crop obvious — a line that runs off
// the edge looks the same cropped or not, a missing tip does not.
func (s *guideSpec) addCross(w, h, stroke int, arrow float64) {
	fw, fh := float64(w), float64(h)
	cx, cy := fw/2, fh/2

	s.rects = append(s.rects, hBand(cy, w, stroke), vBand(cx, h, stroke))

	half := arrow * guideArrowWidthNorm / 2
	s.tris = append(s.tris,
		guideTri{[3][2]float64{{0, cy}, {arrow, cy - half}, {arrow, cy + half}}},            // pointing left
		guideTri{[3][2]float64{{fw, cy}, {fw - arrow, cy - half}, {fw - arrow, cy + half}}}, // right
		guideTri{[3][2]float64{{cx, 0}, {cx - half, arrow}, {cx + half, arrow}}},            // up
		guideTri{[3][2]float64{{cx, fh}, {cx - half, fh - arrow}, {cx + half, fh - arrow}}}, // down
	)
}

// hBand is a full-width band of the given thickness, centred on y.
func hBand(y float64, w, stroke int) guideRect {
	return guideRect{x: 0, y: int(math.Round(y - float64(stroke)/2)), w: w, h: stroke}
}

// vBand is a full-height band of the given thickness, centred on x.
func vBand(x float64, h, stroke int) guideRect {
	return guideRect{x: int(math.Round(x - float64(stroke)/2)), y: 0, w: stroke, h: h}
}

// drawGuides fills spec into img in colour c.
func drawGuides(img *stdimage.RGBA, spec guideSpec, c color.RGBA) {
	for _, r := range spec.rects {
		fillRect(img, r, c)
	}
	for _, t := range spec.tris {
		fillTriangle(img, t, c)
	}
}

func fillRect(img *stdimage.RGBA, r guideRect, c color.RGBA) {
	area := stdimage.Rect(r.x, r.y, r.x+r.w, r.y+r.h).Intersect(img.Bounds())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		row := img.Pix[img.PixOffset(area.Min.X, y):]
		for x := 0; x < area.Dx(); x++ {
			i := x * 4
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

// fillTriangle fills t by testing each pixel centre against the three edges,
// matching the pixel-centre convention the gradient sampler uses. Winding is
// not fixed, so a pixel is inside when it is on the same side of all three.
func fillTriangle(img *stdimage.RGBA, t guideTri, c color.RGBA) {
	minX, minY := t.pts[0][0], t.pts[0][1]
	maxX, maxY := minX, minY
	for _, p := range t.pts[1:] {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}

	area := stdimage.Rect(
		int(math.Floor(minX)), int(math.Floor(minY)),
		int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1,
	).Intersect(img.Bounds())

	for y := area.Min.Y; y < area.Max.Y; y++ {
		py := float64(y) + 0.5
		for x := area.Min.X; x < area.Max.X; x++ {
			px := float64(x) + 0.5
			if !insideTriangle(t, px, py) {
				continue
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

func insideTriangle(t guideTri, px, py float64) bool {
	var neg, pos bool
	for i := range t.pts {
		a, b := t.pts[i], t.pts[(i+1)%3]
		d := (px-a[0])*(b[1]-a[1]) - (py-a[1])*(b[0]-a[0])
		switch {
		case d < 0:
			neg = true
		case d > 0:
			pos = true
		}
	}
	return !neg || !pos
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}

package geo

import "math"

type GeographicPoint struct {
	ID  string
	Lon float64
	Lat float64
}

type ProjectedPoint struct {
	ID string
	X  float64
	Y  float64
}

type ScreenPoint struct {
	ID string
	X  float64
	Y  float64
}

type Bounds struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

type ViewportTransform struct {
	Width   float64
	Height  float64
	Padding float64
	Bounds  Bounds
	Scale   float64
	ScaleX  float64
	ScaleY  float64
	OffsetX float64
	OffsetY float64
}

func ProjectStationCoordinates(points []GeographicPoint) []ProjectedPoint {
	if len(points) == 0 {
		return nil
	}
	minLat, maxLat := math.Inf(1), math.Inf(-1)
	for _, p := range points {
		if !isFinite(p.Lat) || !isFinite(p.Lon) {
			continue
		}
		minLat = math.Min(minLat, p.Lat)
		maxLat = math.Max(maxLat, p.Lat)
	}
	if !isFinite(minLat) || !isFinite(maxLat) {
		return nil
	}
	centerLatRad := ((minLat + maxLat) / 2) * math.Pi / 180
	lonScale := math.Cos(centerLatRad)
	if lonScale == 0 || !isFinite(lonScale) {
		lonScale = 1
	}

	out := make([]ProjectedPoint, 0, len(points))
	for _, p := range points {
		if !isFinite(p.Lat) || !isFinite(p.Lon) {
			continue
		}
		out = append(out, ProjectedPoint{
			ID: p.ID,
			X:  p.Lon * lonScale,
			Y:  p.Lat,
		})
	}
	return out
}

func CalculateViewportTransform(points []ProjectedPoint, width, height, padding float64) ViewportTransform {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	if padding < 0 {
		padding = 0
	}
	maxPadding := math.Min(width, height) / 2
	if padding > maxPadding {
		padding = maxPadding
	}

	b := Bounds{}
	if len(points) > 0 {
		b = Bounds{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
		for _, p := range points {
			if !isFinite(p.X) || !isFinite(p.Y) {
				continue
			}
			b.MinX = math.Min(b.MinX, p.X)
			b.MaxX = math.Max(b.MaxX, p.X)
			b.MinY = math.Min(b.MinY, p.Y)
			b.MaxY = math.Max(b.MaxY, p.Y)
		}
		if !isFinite(b.MinX) || !isFinite(b.MinY) || !isFinite(b.MaxX) || !isFinite(b.MaxY) {
			b = Bounds{}
		}
	}

	sourceWidth := b.MaxX - b.MinX
	sourceHeight := b.MaxY - b.MinY
	drawableWidth := math.Max(0, width-padding*2)
	drawableHeight := math.Max(0, height-padding*2)

	scale := 1.0
	switch {
	case sourceWidth > 0 && sourceHeight > 0:
		scale = math.Min(drawableWidth/sourceWidth, drawableHeight/sourceHeight)
	case sourceWidth > 0:
		scale = drawableWidth / sourceWidth
	case sourceHeight > 0:
		scale = drawableHeight / sourceHeight
	}
	if !isFinite(scale) || scale <= 0 {
		scale = 1
	}

	contentWidth := math.Max(0, sourceWidth) * scale
	contentHeight := math.Max(0, sourceHeight) * scale
	offsetX := padding + (drawableWidth-contentWidth)/2
	offsetY := padding + (drawableHeight-contentHeight)/2

	return ViewportTransform{
		Width:   width,
		Height:  height,
		Padding: padding,
		Bounds:  b,
		Scale:   scale,
		ScaleX:  scale,
		ScaleY:  scale,
		OffsetX: offsetX,
		OffsetY: offsetY,
	}
}

func ApplyViewportTransform(point ProjectedPoint, transform ViewportTransform) ScreenPoint {
	return ScreenPoint{
		ID: point.ID,
		X:  transform.OffsetX + (point.X-transform.Bounds.MinX)*transform.Scale,
		Y:  transform.OffsetY + (transform.Bounds.MaxY-point.Y)*transform.Scale,
	}
}

func ApplyViewportTransformToAll(points []ProjectedPoint, transform ViewportTransform) []ScreenPoint {
	out := make([]ScreenPoint, 0, len(points))
	for _, p := range points {
		out = append(out, ApplyViewportTransform(p, transform))
	}
	return out
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

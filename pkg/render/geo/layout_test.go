package geo

import (
	"math"
	"testing"
)

func TestViewportTransformUsesUniformScaleAcrossViewports(t *testing.T) {
	points := ProjectStationCoordinates([]GeographicPoint{
		{ID: "a", Lon: 135.0, Lat: 34.0},
		{ID: "b", Lon: 136.0, Lat: 34.5},
		{ID: "c", Lon: 137.0, Lat: 35.0},
	})
	if len(points) != 3 {
		t.Fatalf("projected point count = %d, want 3", len(points))
	}

	viewports := []struct {
		width  float64
		height float64
	}{
		{1600, 900},
		{800, 800},
		{900, 1600},
	}

	sourceRatio := projectedRatio(points)
	for _, viewport := range viewports {
		transform := CalculateViewportTransform(points, viewport.width, viewport.height, 40)
		if transform.Scale != transform.ScaleX || transform.Scale != transform.ScaleY {
			t.Fatalf("scale = %f, scaleX = %f, scaleY = %f; want uniform scale", transform.Scale, transform.ScaleX, transform.ScaleY)
		}
		screen := ApplyViewportTransformToAll(points, transform)
		if got := screenRatio(screen); math.Abs(got-sourceRatio) > 1e-9 {
			t.Fatalf("screen ratio = %.12f, source ratio = %.12f for viewport %.0fx%.0f", got, sourceRatio, viewport.width, viewport.height)
		}
	}
}

func TestViewportTransformHonorsPadding(t *testing.T) {
	points := ProjectStationCoordinates([]GeographicPoint{
		{ID: "a", Lon: 135.0, Lat: 34.0},
		{ID: "b", Lon: 136.0, Lat: 34.5},
		{ID: "c", Lon: 137.0, Lat: 35.0},
	})
	const (
		width   = 900.0
		height  = 1600.0
		padding = 80.0
	)
	transform := CalculateViewportTransform(points, width, height, padding)
	screen := ApplyViewportTransformToAll(points, transform)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range screen {
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	if minX < padding || maxX > width-padding || minY < padding || maxY > height-padding {
		t.Fatalf("screen bounds x=[%f,%f] y=[%f,%f], want inside padding %f for %.0fx%.0f", minX, maxX, minY, maxY, padding, width, height)
	}
}

func TestViewportTransformDegenerateInputs(t *testing.T) {
	points := ProjectStationCoordinates([]GeographicPoint{
		{ID: "a", Lon: 135.0, Lat: 34.0},
		{ID: "b", Lon: 135.0, Lat: 34.0},
	})
	transform := CalculateViewportTransform(points, 800, 600, 20)
	screen := ApplyViewportTransformToAll(points, transform)
	for _, p := range screen {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			t.Fatalf("screen point contains invalid value: %+v", p)
		}
	}
}

func projectedRatio(points []ProjectedPoint) float64 {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range points {
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	return (maxX - minX) / (maxY - minY)
}

func screenRatio(points []ScreenPoint) float64 {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range points {
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	return (maxX - minX) / (maxY - minY)
}

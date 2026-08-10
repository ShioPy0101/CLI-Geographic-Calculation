package graphsvg

import (
	"strings"
	"testing"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/railshape"
)

func TestRenderRailGeometriesSVGOutputsOneAnimatedPath(t *testing.T) {
	svg, err := RenderRailGeometriesSVG([]railshape.PathGeometry{
		{Points: []railshape.Point{
			{Lon: 135.0, Lat: 34.0},
			{Lon: 135.1, Lat: 34.05},
			{Lon: 135.2, Lat: 34.0},
		}},
	}, []*graphstructure.Node{
		{ID: "station:a", Kind: "station", Name: "A", Lon: 135.0, Lat: 34.0},
		{ID: "station:b", Kind: "station", Name: "B", Lon: 135.2, Lat: 34.0},
	}, Options{Width: 1200, Height: 800, DrawStations: true, AnimatePath: true})
	if err != nil {
		t.Fatalf("RenderRailGeometriesSVG returned error: %v", err)
	}
	if got := strings.Count(svg, "<path "); got != 1 {
		t.Fatalf("path count = %d, want 1", got)
	}
	for _, want := range []string{"viewBox=", "preserveAspectRatio=\"xMidYMid meet\"", "pathLength=\"1\"", "@keyframes draw-route"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg does not contain %q: %s", want, svg)
		}
	}
}

func TestRenderRailGeometriesSVGCanDisableAnimation(t *testing.T) {
	svg, err := RenderRailGeometriesSVG([]railshape.PathGeometry{
		{Points: []railshape.Point{
			{Lon: 135.0, Lat: 34.0},
			{Lon: 135.1, Lat: 34.05},
		}},
	}, nil, Options{Width: 1200, Height: 800})
	if err != nil {
		t.Fatalf("RenderRailGeometriesSVG returned error: %v", err)
	}
	if strings.Contains(svg, "@keyframes draw-route") || strings.Contains(svg, "pathLength=\"1\"") || strings.Contains(svg, "class=\"rail-path\"") {
		t.Fatalf("static svg contains animation markup: %s", svg)
	}
}

func TestRenderRailGeometriesSVGCanDisableLabels(t *testing.T) {
	svg, err := RenderRailGeometriesSVG([]railshape.PathGeometry{
		{Points: []railshape.Point{
			{Lon: 135.0, Lat: 34.0},
			{Lon: 135.1, Lat: 34.05},
		}},
	}, []*graphstructure.Node{
		{ID: "station:a", Kind: "station", Name: "上郡", Lon: 135.0, Lat: 34.0},
	}, Options{Width: 1200, Height: 800, DrawStations: true, DrawLabels: false})
	if err != nil {
		t.Fatalf("RenderRailGeometriesSVG returned error: %v", err)
	}
	if strings.Contains(svg, "<text ") {
		t.Fatalf("label-disabled svg contains text: %s", svg)
	}
}

func TestRenderRailGeometriesSVGUsesJapaneseFontFamily(t *testing.T) {
	svg, err := RenderRailGeometriesSVG([]railshape.PathGeometry{
		{Points: []railshape.Point{
			{Lon: 135.0, Lat: 34.0},
			{Lon: 135.1, Lat: 34.05},
		}},
	}, []*graphstructure.Node{
		{ID: "station:a", Kind: "station", Name: "上郡", Lon: 135.0, Lat: 34.0},
	}, Options{Width: 1200, Height: 800, DrawStations: true, DrawLabels: true})
	if err != nil {
		t.Fatalf("RenderRailGeometriesSVG returned error: %v", err)
	}
	if !strings.Contains(svg, "Noto Sans CJK JP") {
		t.Fatalf("svg does not contain Japanese font family: %s", svg)
	}
	if strings.Contains(svg, "Arial") {
		t.Fatalf("svg contains Arial fallback: %s", svg)
	}
}

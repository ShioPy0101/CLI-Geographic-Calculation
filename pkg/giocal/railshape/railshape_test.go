package railshape

import (
	"path/filepath"
	"runtime"
	"testing"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
)

func TestLoadRealRailroadShapeGeoJSON(t *testing.T) {
	resolver, err := Load(realShapePath(t))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	features := resolver.featuresByLine["山陽線"]
	if len(features) == 0 {
		t.Fatal("山陽線 features not found")
	}
	if got := features[0].Geometry.Type; got != "LineString" {
		t.Fatalf("geometry type = %q, want LineString", got)
	}
	if propertyString(features[0].Properties, LineProperty) == "" {
		t.Fatalf("line property %s is empty", LineProperty)
	}
	if propertyString(features[0].Properties, OperatorProperty) == "" {
		t.Fatalf("operator property %s is empty", OperatorProperty)
	}
	if propertyString(features[0].Properties, SectionProperty) == "" {
		t.Fatalf("section property %s is empty", SectionProperty)
	}
}

func TestFeatureFilteringUsesActualProperties(t *testing.T) {
	resolver, err := Load(realShapePath(t))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	segment := routepath.RouteSegment{
		LineID: "山陽線",
		Stations: []*graphstructure.Node{
			{Kind: "station", Name: "上郡", Meta: map[string]string{"company": "西日本旅客鉄道"}},
		},
	}
	features := resolver.featuresForSegment(segment)
	if len(features) == 0 {
		t.Fatal("filtered features are empty")
	}
	for _, feature := range features {
		if got := propertyString(feature.Properties, LineProperty); got != "山陽線" {
			t.Fatalf("line property = %q, want 山陽線", got)
		}
		if got := normalizeOperator(propertyString(feature.Properties, OperatorProperty)); got != "西日本旅客鉄道" {
			t.Fatalf("operator property = %q, want 西日本旅客鉄道", got)
		}
	}
}

func TestMergeFeaturesOrdersConnectedLineStrings(t *testing.T) {
	features := []Feature{
		testFeature("Test", "Op", []Point{{Lon: 1, Lat: 0}, {Lon: 2, Lat: 0}}),
		testFeature("Test", "Op", []Point{{Lon: 3, Lat: 0}, {Lon: 2, Lat: 0}}),
		testFeature("Test", "Op", []Point{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 0}}),
	}
	components := mergeFeatures(features)
	if len(components) != 1 {
		t.Fatalf("component count = %d, want 1", len(components))
	}
	got := components[0]
	if len(got) != 4 || got[0].Lon != 0 || got[3].Lon != 3 {
		t.Fatalf("merged component = %+v, want 0 -> 3", got)
	}
}

func TestNearestPositionProjectsToPolylineSegment(t *testing.T) {
	polyline := []Point{{Lon: 0, Lat: 0}, {Lon: 10, Lat: 0}}
	pos := nearestPosition(polyline, Point{Lon: 4, Lat: 3})
	if !pos.Valid {
		t.Fatal("position is invalid")
	}
	if pos.SegmentIndex != 0 {
		t.Fatalf("segment index = %d, want 0", pos.SegmentIndex)
	}
	if pos.T < 0.39 || pos.T > 0.41 {
		t.Fatalf("t = %f, want about 0.4", pos.T)
	}
	if pos.Point.Lon < 3.99 || pos.Point.Lon > 4.01 || pos.Point.Lat != 0 {
		t.Fatalf("nearest point = %+v, want projection on segment", pos.Point)
	}
}

func TestRouteSlicingAndReverse(t *testing.T) {
	polyline := []Point{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 0}, {Lon: 2, Lat: 0}, {Lon: 3, Lat: 0}}
	start := nearestPosition(polyline, Point{Lon: 2.5, Lat: 0})
	end := nearestPosition(polyline, Point{Lon: 0.5, Lat: 0})
	out := slicePolyline(polyline, start, end)
	if len(out) != 4 {
		t.Fatalf("sliced point count = %d, want 4: %+v", len(out), out)
	}
	if out[0].Lon != 2.5 || out[len(out)-1].Lon != 0.5 {
		t.Fatalf("sliced route = %+v, want reversed 2.5 -> 0.5", out)
	}
}

func TestResolveRealSanyoSegmentUsesDenseShape(t *testing.T) {
	resolver, err := Load(realShapePath(t))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	geometry, err := resolver.ResolveSegment(routepath.RouteSegment{
		LineID: "山陽線",
		Stations: []*graphstructure.Node{
			{Kind: "station", Name: "上郡", Lon: 134.35324, Lat: 34.86599, Meta: map[string]string{"company": "西日本旅客鉄道"}},
			{Kind: "station", Name: "神戸", Lon: 135.17838, Lat: 34.68057, Meta: map[string]string{"company": "西日本旅客鉄道"}},
		},
	})
	if err != nil {
		t.Fatalf("ResolveSegment returned error: %v", err)
	}
	if len(geometry.Points) < 100 {
		t.Fatalf("geometry points = %d, want dense railroad shape", len(geometry.Points))
	}
}

func TestResolveRealKamigoriKyotoSplitRouteCanFlatten(t *testing.T) {
	resolver, err := Load(realShapePath(t))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	route := routepath.ResolvedRoute{Segments: []routepath.RouteSegment{
		{
			LineID: "山陽線",
			Stations: []*graphstructure.Node{
				{Kind: "station", Name: "上郡", Lon: 134.35324, Lat: 34.86599, Meta: map[string]string{"company": "西日本旅客鉄道"}},
				{Kind: "station", Name: "神戸", Lon: 135.17838, Lat: 34.68057, Meta: map[string]string{"company": "西日本旅客鉄道"}},
			},
		},
		{
			LineID: "東海道線",
			Stations: []*graphstructure.Node{
				{Kind: "station", Name: "神戸", Lon: 135.17838, Lat: 34.68057, Meta: map[string]string{"company": "西日本旅客鉄道"}},
				{Kind: "station", Name: "京都", Lon: 135.75877, Lat: 34.98585, Meta: map[string]string{"company": "西日本旅客鉄道"}},
			},
		},
	}}
	paths, err := resolver.Resolve(route)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if _, err := FlattenContinuous(paths); err != nil {
		t.Fatalf("FlattenContinuous returned error: %v", err)
	}
}

func testFeature(line, operator string, points []Point) Feature {
	coords := make([][]float64, 0, len(points))
	for _, point := range points {
		coords = append(coords, []float64{point.Lon, point.Lat})
	}
	return Feature{
		Type: "Feature",
		Properties: map[string]any{
			LineProperty:     line,
			OperatorProperty: operator,
			SectionProperty:  "section",
		},
		Geometry: Geometry{Type: "LineString", Coordinates: coords},
	}
}

func realShapePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "giodata", "N05-24_RailroadSection2.geojson"))
}

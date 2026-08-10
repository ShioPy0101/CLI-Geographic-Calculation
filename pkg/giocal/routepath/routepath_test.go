package routepath

import (
	"strings"
	"testing"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
)

func TestFlattenContinuousRouteBasicAndBoundaryDedup(t *testing.T) {
	path, err := FlattenContinuousRoute(ResolvedRoute{Segments: []RouteSegment{
		{LineID: "L1", Stations: stations("A", "B", "C")},
		{LineID: "L2", Stations: stations("C", "D", "E")},
	}}, FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	assertStationOrder(t, path, []string{"A", "B", "C", "D", "E"})
}

func TestFlattenContinuousRouteReversePossible(t *testing.T) {
	path, err := FlattenContinuousRoute(ResolvedRoute{Segments: []RouteSegment{
		{LineID: "L1", Stations: stations("A", "B", "C")},
		{LineID: "L2", Stations: stations("E", "D", "C")},
	}}, FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	assertStationOrder(t, path, []string{"A", "B", "C", "D", "E"})
}

func TestFlattenContinuousRouteConnectsSameNameSameCoordinateDifferentIDs(t *testing.T) {
	kobeA := &graphstructure.Node{ID: "station:sanyo:kobe", Kind: "station", Name: "神戸", Lon: 135.17838, Lat: 34.68057}
	kobeB := &graphstructure.Node{ID: "station:tokaido:kobe", Kind: "station", Name: "神戸", Lon: 135.17838, Lat: 34.68057}
	path, err := FlattenContinuousRoute(ResolvedRoute{Segments: []RouteSegment{
		{LineID: "山陽線", Stations: []*graphstructure.Node{
			{ID: "station:kamigori", Kind: "station", Name: "上郡", Lon: 134.35, Lat: 34.87},
			kobeA,
		}},
		{LineID: "東海道線", Stations: []*graphstructure.Node{
			kobeB,
			{ID: "station:osaka", Kind: "station", Name: "大阪", Lon: 135.49, Lat: 34.70},
		}},
	}}, FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	assertStationOrder(t, path, []string{"上郡", "神戸", "大阪"})
}

func TestFlattenContinuousRouteRejectsDisconnectedSegments(t *testing.T) {
	_, err := FlattenContinuousRoute(ResolvedRoute{Segments: []RouteSegment{
		{LineID: "L1", Stations: stations("A", "B", "C")},
		{LineID: "L2", Stations: stations("X", "Y", "Z")},
	}}, FlattenOptions{AllowReverse: true})
	if err == nil {
		t.Fatal("FlattenContinuousRoute returned nil error")
	}
	if !strings.Contains(err.Error(), "segment 2 does not connect to the current route") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestFlattenContinuousRouteKeepsLoopStations(t *testing.T) {
	path, err := FlattenContinuousRoute(ResolvedRoute{Segments: []RouteSegment{
		{LineID: "L1", Stations: stations("A", "B", "C")},
		{LineID: "L2", Stations: stations("C", "D", "B")},
		{LineID: "L3", Stations: stations("B", "E")},
	}}, FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	assertStationOrder(t, path, []string{"A", "B", "C", "D", "B", "E"})
}

func stations(names ...string) []*graphstructure.Node {
	out := make([]*graphstructure.Node, 0, len(names))
	for _, name := range names {
		out = append(out, &graphstructure.Node{ID: "station:" + name, Kind: "station", Name: name})
	}
	return out
}

func assertStationOrder(t *testing.T, path RenderPath, want []string) {
	t.Helper()
	if len(path.Stations) != len(want) {
		t.Fatalf("station count = %d, want %d: %+v", len(path.Stations), len(want), path.Stations)
	}
	for i, station := range path.Stations {
		if station.Name != want[i] {
			t.Fatalf("station[%d] = %q, want %q", i, station.Name, want[i])
		}
	}
}

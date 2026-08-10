package sqlreq

import (
	"encoding/json"
	"strings"
	"testing"

	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/linefilter"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
)

func TestParseRouteSelectionQuery(t *testing.T) {
	selections, err := ParseRouteSelectionQuery(`SELECT 山手線 BETWEEN "新宿" AND "東京", 総武線;`)
	if err != nil {
		t.Fatalf("ParseRouteSelectionQuery returned error: %v", err)
	}
	if len(selections) != 2 {
		t.Fatalf("selection count = %d, want 2", len(selections))
	}
	if selections[0].Line != "山手線" || *selections[0].FromStation != "新宿" || *selections[0].ToStation != "東京" {
		t.Fatalf("unexpected first selection: %+v", selections[0])
	}
	if selections[1].Line != "総武線" || selections[1].FromStation != nil || selections[1].ToStation != nil {
		t.Fatalf("unexpected second selection: %+v", selections[1])
	}
}

func TestParseRouteQuerySingleLineOption(t *testing.T) {
	query, err := ParseRouteQuery(`SELECT Alpha BETWEEN A AND B, Beta OPTION geographic, single_line, no_animation, no_labels, endpoint_labels;`)
	if err != nil {
		t.Fatalf("ParseRouteQuery returned error: %v", err)
	}
	if !query.Options.SingleLine {
		t.Fatal("SingleLine = false, want true")
	}
	if len(query.Selections) != 2 {
		t.Fatalf("selection count = %d, want 2", len(query.Selections))
	}
	if !query.Options.Geographic {
		t.Fatal("Geographic = false, want true")
	}
	if !query.Options.NoAnimation {
		t.Fatal("NoAnimation = false, want true")
	}
	if !query.Options.NoLabels {
		t.Fatal("NoLabels = false, want true")
	}
	if !query.Options.EndpointLabels {
		t.Fatal("EndpointLabels = false, want true")
	}
}

func TestRouteSelectionsToGraphWholeLine(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"A", "B", "C", "D"})
	assertRailEdgeCount(t, graph, "Alpha", 3)
}

func TestRouteSelectionsToGraphBetween(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha BETWEEN B AND D;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"B", "C", "D"})
	assertRailEdgeCount(t, graph, "Alpha", 2)
}

func TestSQLLikeToResolvedRouteSingleLineOption(t *testing.T) {
	resolved, options, err := SQLLikeToResolvedRoute("SELECT Alpha BETWEEN A AND B, Alpha BETWEEN B AND D OPTION single_line;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToResolvedRoute returned error: %v", err)
	}
	if !options.SingleLine {
		t.Fatal("SingleLine = false, want true")
	}
	path, err := routepath.FlattenContinuousRoute(resolved, routepath.FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	got := []string{}
	for _, station := range path.Stations {
		got = append(got, station.Name)
	}
	want := []string{"A", "B", "C", "D"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("stations = %v, want %v", got, want)
	}
}

func TestRouteSelectionSplitsThroughAdjacentLine(t *testing.T) {
	resolved, _, err := SQLLikeToResolvedRoute("SELECT Alpha BETWEEN B AND E OPTION single_line;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToResolvedRoute returned error: %v", err)
	}
	if len(resolved.Segments) != 2 {
		t.Fatalf("segment count = %d, want 2", len(resolved.Segments))
	}
	if resolved.Segments[0].LineID != "Alpha" || resolved.Segments[1].LineID != "Delta" {
		t.Fatalf("segment lines = %q, %q; want Alpha, Delta", resolved.Segments[0].LineID, resolved.Segments[1].LineID)
	}
	path, err := routepath.FlattenContinuousRoute(resolved, routepath.FlattenOptions{AllowReverse: true})
	if err != nil {
		t.Fatalf("FlattenContinuousRoute returned error: %v", err)
	}
	got := []string{}
	for _, station := range path.Stations {
		got = append(got, station.Name)
	}
	want := []string{"B", "C", "D", "E"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("stations = %v, want %v", got, want)
	}
}

func TestRouteSelectionsToGraphSplitsThroughAdjacentLine(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha BETWEEN B AND E;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"B", "C", "D", "E"})
	assertRailEdgeCount(t, graph, "Alpha", 2)
	assertRailEdgeCount(t, graph, "Delta", 1)
}

func TestRouteSelectionsToGraphMultipleAllBetween(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha BETWEEN A AND B, Beta BETWEEN X AND Z;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"A", "B", "X", "Y", "Z"})
	assertRailEdgeCount(t, graph, "Alpha", 1)
	assertRailEdgeCount(t, graph, "Beta", 2)
}

func TestRouteSelectionsToGraphMixedWholeLineAndBetween(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha BETWEEN B AND C, Beta;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"B", "C", "X", "Y", "Z"})
	assertRailEdgeCount(t, graph, "Alpha", 1)
	assertRailEdgeCount(t, graph, "Beta", 2)
}

func TestRouteSelectionsToGraphBetweenForwardAndReverse(t *testing.T) {
	for _, query := range []string{
		"SELECT Alpha BETWEEN A AND D;",
		"SELECT Alpha BETWEEN D AND A;",
	} {
		graph, err := SQLLikeToGraph(query, testDatasetResource(t))
		if err != nil {
			t.Fatalf("%s returned error: %v", query, err)
		}
		assertStationNames(t, graph, []string{"A", "B", "C", "D"})
		assertRailEdgeCount(t, graph, "Alpha", 3)
	}
}

func TestRouteSelectionsToGraphBetweenSameStation(t *testing.T) {
	graph, err := SQLLikeToGraph("SELECT Alpha BETWEEN B AND B;", testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"B"})
	assertRailEdgeCount(t, graph, "Alpha", 0)
}

func TestRouteSelectionsToGraphValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr string
	}{
		{
			name:    "missing from station",
			query:   "SELECT Alpha BETWEEN Missing AND B;",
			wantErr: `from station "Missing" does not exist`,
		},
		{
			name:    "missing to station",
			query:   "SELECT Alpha BETWEEN A AND Missing;",
			wantErr: `to station "Missing" does not exist`,
		},
		{
			name:    "station exists but not on line",
			query:   "SELECT Alpha BETWEEN A AND Outside;",
			wantErr: `to station "Outside" does not exist on line "Alpha"`,
		},
		{
			name:    "missing line",
			query:   "SELECT MissingLine;",
			wantErr: `line "MissingLine" does not exist`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SQLLikeToGraph(tt.query, testDatasetResource(t))
			if err == nil {
				t.Fatal("SQLLikeToGraph returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRouteSelectionsToGraphQuotedStationName(t *testing.T) {
	graph, err := SQLLikeToGraph(`SELECT Gamma BETWEEN "A terminal" AND "C-hub";`, testDatasetResource(t))
	if err != nil {
		t.Fatalf("SQLLikeToGraph returned error: %v", err)
	}
	assertStationNames(t, graph, []string{"A terminal", "C-hub"})
	assertRailEdgeCount(t, graph, "Gamma", 2)
}

func TestExistingSQLRegression(t *testing.T) {
	parsed := ParseSQLQuery("SELECT * FROM rail WHERE company = 'JR Test' AND line IN ('Alpha')")
	graph := SQLToGraph(linefilter.FilterRailroadSectionByProperties, parsed, testDatasetResource(t))
	assertStationNames(t, graph, []string{"A", "B", "C", "D"})
	assertRailEdgeCount(t, graph, "Alpha", 3)
}

func assertStationNames(t *testing.T, graph *graphstructure.Graph, want []string) {
	t.Helper()
	got := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind == "station" {
			got[node.Name] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("station names = %+v, want %v", got, want)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("station names = %+v, missing %q", got, name)
		}
	}
}

func assertRailEdgeCount(t *testing.T, graph *graphstructure.Graph, line string, want int) {
	t.Helper()
	got := 0
	for _, edge := range graph.Edges {
		if edge.Kind == "rail" && edge.Meta["line"] == line {
			got++
		}
	}
	if got != want {
		t.Fatalf("rail edge count for %s = %d, want %d", line, got, want)
	}
}

func testDatasetResource(t *testing.T) *giocaltype.DatasetResource {
	t.Helper()
	railJSON := `{
		"type": "FeatureCollection",
		"features": [
			{"type":"Feature","properties":{"N02_003":"Alpha","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[0,0],[1,0],[2,0],[3,0]]}},
			{"type":"Feature","properties":{"N02_003":"Beta","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[10,0],[11,0],[12,0]]}},
			{"type":"Feature","properties":{"N02_003":"Other","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[20,0],[21,0]]}},
			{"type":"Feature","properties":{"N02_003":"Gamma","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[30,0],[31,0],[32,0]]}},
			{"type":"Feature","properties":{"N02_003":"Aardvark","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[100,0],[101,0]]}},
			{"type":"Feature","properties":{"N02_003":"Delta","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[3,0],[4,0],[5,0]]}}
		]
	}`
	stationJSON := `{
		"type": "FeatureCollection",
		"features": [
			{"type":"Feature","properties":{"N02_003":"Alpha","N02_004":"JR Test","N02_005":"A","N02_005c":"A"},"geometry":{"type":"LineString","coordinates":[[0,0]]}},
			{"type":"Feature","properties":{"N02_003":"Alpha","N02_004":"JR Test","N02_005":"B","N02_005c":"B"},"geometry":{"type":"LineString","coordinates":[[1,0]]}},
			{"type":"Feature","properties":{"N02_003":"Alpha","N02_004":"JR Test","N02_005":"C","N02_005c":"C"},"geometry":{"type":"LineString","coordinates":[[2,0]]}},
			{"type":"Feature","properties":{"N02_003":"Alpha","N02_004":"JR Test","N02_005":"D","N02_005c":"D"},"geometry":{"type":"LineString","coordinates":[[3,0]]}},
			{"type":"Feature","properties":{"N02_003":"Beta","N02_004":"JR Test","N02_005":"X","N02_005c":"X"},"geometry":{"type":"LineString","coordinates":[[10,0]]}},
			{"type":"Feature","properties":{"N02_003":"Beta","N02_004":"JR Test","N02_005":"Y","N02_005c":"Y"},"geometry":{"type":"LineString","coordinates":[[11,0]]}},
			{"type":"Feature","properties":{"N02_003":"Beta","N02_004":"JR Test","N02_005":"Z","N02_005c":"Z"},"geometry":{"type":"LineString","coordinates":[[12,0]]}},
			{"type":"Feature","properties":{"N02_003":"Other","N02_004":"JR Test","N02_005":"Outside","N02_005c":"Outside"},"geometry":{"type":"LineString","coordinates":[[20,0]]}},
			{"type":"Feature","properties":{"N02_003":"Gamma","N02_004":"JR Test","N02_005":"A terminal","N02_005c":"A-terminal"},"geometry":{"type":"LineString","coordinates":[[30,0]]}},
			{"type":"Feature","properties":{"N02_003":"Gamma","N02_004":"JR Test","N02_005":"C-hub","N02_005c":"C-hub"},"geometry":{"type":"LineString","coordinates":[[32,0]]}},
			{"type":"Feature","properties":{"N02_003":"Aardvark","N02_004":"JR Test","N02_005":"D","N02_005c":"D-far"},"geometry":{"type":"LineString","coordinates":[[100,0]]}},
			{"type":"Feature","properties":{"N02_003":"Aardvark","N02_004":"JR Test","N02_005":"E","N02_005c":"E-far"},"geometry":{"type":"LineString","coordinates":[[101,0]]}},
			{"type":"Feature","properties":{"N02_003":"Delta","N02_004":"JR Test","N02_005":"D","N02_005c":"D"},"geometry":{"type":"LineString","coordinates":[[3,0]]}},
			{"type":"Feature","properties":{"N02_003":"Delta","N02_004":"JR Test","N02_005":"E","N02_005c":"E"},"geometry":{"type":"LineString","coordinates":[[4,0]]}}
		]
	}`
	var rail giocaltype.GiotypeRailroadSectionFeatureCollection
	var station giocaltype.GiotypeStationFeatureCollection
	if err := json.Unmarshal([]byte(railJSON), &rail); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(stationJSON), &station); err != nil {
		t.Fatal(err)
	}
	return &giocaltype.DatasetResource{Rail: &rail, Station: &station}
}

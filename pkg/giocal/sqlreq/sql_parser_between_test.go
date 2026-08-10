package sqlreq

import (
	"encoding/json"
	"strings"
	"testing"

	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/linefilter"
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
			{"type":"Feature","properties":{"N02_003":"Gamma","N02_004":"JR Test"},"geometry":{"type":"LineString","coordinates":[[30,0],[31,0],[32,0]]}}
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
			{"type":"Feature","properties":{"N02_003":"Gamma","N02_004":"JR Test","N02_005":"C-hub","N02_005c":"C-hub"},"geometry":{"type":"LineString","coordinates":[[32,0]]}}
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

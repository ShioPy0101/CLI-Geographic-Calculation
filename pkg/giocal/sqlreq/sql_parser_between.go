package sqlreq

import (
	"container/heap"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"CLI-Geographic-Calculation/pkg/giocal"
	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
)

type RouteSelection struct {
	Line        string  `json:"line"`
	FromStation *string `json:"from_station"`
	ToStation   *string `json:"to_station"`
}

func (s RouteSelection) HasStationRange() bool {
	return s.FromStation != nil || s.ToStation != nil
}

type routeTokenKind int

const (
	routeTokenEOF routeTokenKind = iota
	routeTokenIdent
	routeTokenString
	routeTokenSelect
	routeTokenBetween
	routeTokenAnd
	routeTokenComma
	routeTokenSemicolon
)

type routeToken struct {
	kind routeTokenKind
	text string
	pos  int
}

func ParseRouteSelectionQuery(query string) ([]RouteSelection, error) {
	tokens, err := lexRouteSelectionQuery(query)
	if err != nil {
		return nil, err
	}
	p := routeSelectionParser{tokens: tokens}
	return p.parse()
}

func SQLLikeToGraph(query string, drs *giocaltype.DatasetResource) (*graphstructure.Graph, error) {
	selections, err := ParseRouteSelectionQuery(query)
	if err != nil {
		return nil, err
	}
	return RouteSelectionsToGraph(selections, drs)
}

func lexRouteSelectionQuery(query string) ([]routeToken, error) {
	tokens := []routeToken{}
	for pos := 0; pos < len(query); {
		r, size := utf8.DecodeRuneInString(query[pos:])
		if unicode.IsSpace(r) {
			pos += size
			continue
		}
		switch r {
		case ',':
			tokens = append(tokens, routeToken{kind: routeTokenComma, text: ",", pos: pos})
			pos += size
			continue
		case ';':
			tokens = append(tokens, routeToken{kind: routeTokenSemicolon, text: ";", pos: pos})
			pos += size
			continue
		case '"', '\'':
			text, next, err := readQuotedRouteToken(query, pos, r)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, routeToken{kind: routeTokenString, text: text, pos: pos})
			pos = next
			continue
		}

		start := pos
		for pos < len(query) {
			r, size = utf8.DecodeRuneInString(query[pos:])
			if unicode.IsSpace(r) || r == ',' || r == ';' || r == '"' || r == '\'' {
				break
			}
			pos += size
		}
		text := query[start:pos]
		switch strings.ToUpper(text) {
		case "SELECT":
			tokens = append(tokens, routeToken{kind: routeTokenSelect, text: text, pos: start})
		case "BETWEEN":
			tokens = append(tokens, routeToken{kind: routeTokenBetween, text: text, pos: start})
		case "AND":
			tokens = append(tokens, routeToken{kind: routeTokenAnd, text: text, pos: start})
		default:
			tokens = append(tokens, routeToken{kind: routeTokenIdent, text: text, pos: start})
		}
	}
	tokens = append(tokens, routeToken{kind: routeTokenEOF, pos: len(query)})
	return tokens, nil
}

func readQuotedRouteToken(query string, start int, quote rune) (string, int, error) {
	var b strings.Builder
	pos := start + utf8.RuneLen(quote)
	for pos < len(query) {
		r, size := utf8.DecodeRuneInString(query[pos:])
		if r == quote {
			next := pos + size
			if next < len(query) {
				nextR, nextSize := utf8.DecodeRuneInString(query[next:])
				if nextR == quote {
					b.WriteRune(quote)
					pos = next + nextSize
					continue
				}
			}
			return b.String(), next, nil
		}
		if r == '\\' {
			next := pos + size
			if next < len(query) {
				escaped, escapedSize := utf8.DecodeRuneInString(query[next:])
				b.WriteRune(escaped)
				pos = next + escapedSize
				continue
			}
		}
		b.WriteRune(r)
		pos += size
	}
	return "", pos, fmt.Errorf("unterminated string literal at byte %d", start)
}

type routeSelectionParser struct {
	tokens []routeToken
	pos    int
}

func (p *routeSelectionParser) parse() ([]RouteSelection, error) {
	if _, err := p.expect(routeTokenSelect); err != nil {
		return nil, err
	}
	selections := []RouteSelection{}
	for {
		selection, err := p.parseSelection()
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)

		if p.match(routeTokenComma) {
			continue
		}
		break
	}
	p.match(routeTokenSemicolon)
	if p.peek().kind != routeTokenEOF {
		return nil, fmt.Errorf("unexpected token %q at byte %d", p.peek().text, p.peek().pos)
	}
	return selections, nil
}

func (p *routeSelectionParser) parseSelection() (RouteSelection, error) {
	line, err := p.expectName("line")
	if err != nil {
		return RouteSelection{}, err
	}
	selection := RouteSelection{Line: line.text}
	if !p.match(routeTokenBetween) {
		return selection, nil
	}
	from, err := p.expectName("from station")
	if err != nil {
		return RouteSelection{}, err
	}
	if _, err := p.expect(routeTokenAnd); err != nil {
		return RouteSelection{}, err
	}
	to, err := p.expectName("to station")
	if err != nil {
		return RouteSelection{}, err
	}
	selection.FromStation = &from.text
	selection.ToStation = &to.text
	return selection, nil
}

func (p *routeSelectionParser) expectName(label string) (routeToken, error) {
	tok := p.peek()
	if tok.kind != routeTokenIdent && tok.kind != routeTokenString {
		return routeToken{}, fmt.Errorf("expected %s at byte %d", label, tok.pos)
	}
	p.pos++
	return tok, nil
}

func (p *routeSelectionParser) expect(kind routeTokenKind) (routeToken, error) {
	tok := p.peek()
	if tok.kind != kind {
		return routeToken{}, fmt.Errorf("expected %s at byte %d", routeTokenKindName(kind), tok.pos)
	}
	p.pos++
	return tok, nil
}

func (p *routeSelectionParser) match(kind routeTokenKind) bool {
	if p.peek().kind != kind {
		return false
	}
	p.pos++
	return true
}

func (p *routeSelectionParser) peek() routeToken {
	if p.pos >= len(p.tokens) {
		return routeToken{kind: routeTokenEOF}
	}
	return p.tokens[p.pos]
}

func routeTokenKindName(kind routeTokenKind) string {
	switch kind {
	case routeTokenSelect:
		return "SELECT"
	case routeTokenBetween:
		return "BETWEEN"
	case routeTokenAnd:
		return "AND"
	case routeTokenComma:
		return ","
	case routeTokenSemicolon:
		return ";"
	case routeTokenEOF:
		return "EOF"
	default:
		return "identifier"
	}
}

func RouteSelectionsToGraph(selections []RouteSelection, drs *giocaltype.DatasetResource) (*graphstructure.Graph, error) {
	if len(selections) == 0 {
		return nil, errors.New("at least one route selection is required")
	}
	out := graphstructure.NewGraph()
	for _, selection := range selections {
		if selection.Line == "" {
			return nil, errors.New("line name is required")
		}
		lineRailIndices := railIndicesForLine(drs, selection.Line)
		if len(lineRailIndices) == 0 {
			return nil, fmt.Errorf("line %q does not exist", selection.Line)
		}
		lineStationIndices := stationIndicesForLine(drs, selection.Line)
		if !selection.HasStationRange() {
			mergeGraph(out, giocal.ConvertGiotypeRailwayToGraphByRequired(drs.Station, drs.Rail, lineStationIndices, lineRailIndices))
			continue
		}
		if selection.FromStation == nil || selection.ToStation == nil {
			return nil, fmt.Errorf("line %q must specify both from_station and to_station", selection.Line)
		}
		if !stationExists(drs, *selection.FromStation) {
			return nil, fmt.Errorf("from station %q does not exist", *selection.FromStation)
		}
		if !stationExists(drs, *selection.ToStation) {
			return nil, fmt.Errorf("to station %q does not exist", *selection.ToStation)
		}
		if !stationExistsOnLine(drs, selection.Line, *selection.FromStation) {
			return nil, fmt.Errorf("from station %q does not exist on line %q", *selection.FromStation, selection.Line)
		}
		if !stationExistsOnLine(drs, selection.Line, *selection.ToStation) {
			return nil, fmt.Errorf("to station %q does not exist on line %q", *selection.ToStation, selection.Line)
		}

		lineGraph := giocal.ConvertGiotypeRailwayToGraphByRequired(drs.Station, drs.Rail, lineStationIndices, lineRailIndices)
		sectionGraph, err := extractStationRangeGraph(lineGraph, selection.Line, *selection.FromStation, *selection.ToStation)
		if err != nil {
			return nil, err
		}
		mergeGraph(out, sectionGraph)
	}
	return out, nil
}

func railIndicesForLine(drs *giocaltype.DatasetResource, line string) []int {
	out := []int{}
	for i, rail := range drs.Rail.Features {
		if rail.Properties.N02003 == line {
			out = append(out, i)
		}
	}
	return out
}

func stationIndicesForLine(drs *giocaltype.DatasetResource, line string) []int {
	out := []int{}
	for i, station := range drs.Station.Features {
		if station.Properties.N02003 == line {
			out = append(out, i)
		}
	}
	return out
}

func stationExists(drs *giocaltype.DatasetResource, stationName string) bool {
	for _, station := range drs.Station.Features {
		if station.Properties.N02005 == stationName {
			return true
		}
	}
	return false
}

func stationExistsOnLine(drs *giocaltype.DatasetResource, line, stationName string) bool {
	for _, station := range drs.Station.Features {
		if station.Properties.N02003 == line && station.Properties.N02005 == stationName {
			return true
		}
	}
	return false
}

func extractStationRangeGraph(g *graphstructure.Graph, line, fromStation, toStation string) (*graphstructure.Graph, error) {
	fromIDs := stationNodeIDs(g, line, fromStation)
	toIDs := stationNodeIDs(g, line, toStation)
	if len(fromIDs) == 0 {
		return nil, fmt.Errorf("from station %q was not found in graph for line %q", fromStation, line)
	}
	if len(toIDs) == 0 {
		return nil, fmt.Errorf("to station %q was not found in graph for line %q", toStation, line)
	}

	bestPath := []string(nil)
	bestDist := routeInf
	for _, fromID := range fromIDs {
		for _, toID := range toIDs {
			path, dist, ok := shortestPath(g, fromID, toID)
			if ok && dist < bestDist {
				bestPath = path
				bestDist = dist
			}
		}
	}
	if len(bestPath) == 0 {
		return nil, fmt.Errorf("no path found on line %q between %q and %q", line, fromStation, toStation)
	}

	out := graphstructure.NewGraph()
	pathNodeIDs := map[string]bool{}
	for _, id := range bestPath {
		pathNodeIDs[id] = true
		if node := g.Nodes[id]; node != nil {
			out.Nodes[id] = node
		}
	}
	for i := 1; i < len(bestPath); i++ {
		if edge := findEdgeBetween(g, bestPath[i-1], bestPath[i]); edge != nil {
			out.Edges = append(out.Edges, edge)
		}
	}
	addStationsAttachedToPath(g, out, pathNodeIDs, line)
	return out, nil
}

func stationNodeIDs(g *graphstructure.Graph, line, stationName string) []string {
	out := []string{}
	for id, node := range g.Nodes {
		if node.Kind == "station" && node.Name == stationName && node.Meta["line"] == line {
			out = append(out, id)
		}
	}
	return out
}

func addStationsAttachedToPath(source, dest *graphstructure.Graph, pathNodeIDs map[string]bool, line string) {
	for _, edge := range source.Edges {
		if edge.Kind != "station_at" || edge.Meta["line"] != line {
			continue
		}
		stationID, coordID := "", ""
		if source.Nodes[edge.From] != nil && source.Nodes[edge.From].Kind == "station" {
			stationID, coordID = edge.From, edge.To
		} else if source.Nodes[edge.To] != nil && source.Nodes[edge.To].Kind == "station" {
			stationID, coordID = edge.To, edge.From
		}
		if stationID == "" || !pathNodeIDs[coordID] {
			continue
		}
		dest.Nodes[stationID] = source.Nodes[stationID]
		if dest.Nodes[coordID] != nil {
			dest.Edges = append(dest.Edges, edge)
		}
	}
}

func mergeGraph(dest, source *graphstructure.Graph) {
	for id, node := range source.Nodes {
		dest.Nodes[id] = node
	}
	seen := map[string]bool{}
	for _, edge := range dest.Edges {
		seen[edgeKey(edge)] = true
	}
	for _, edge := range source.Edges {
		key := edgeKey(edge)
		if seen[key] {
			continue
		}
		dest.Edges = append(dest.Edges, edge)
		seen[key] = true
	}
}

func edgeKey(edge *graphstructure.Edge) string {
	return edge.From + "\x00" + edge.To + "\x00" + edge.Kind + "\x00" + edge.Meta["line"]
}

const routeInf = float64(1 << 60)

type routeAdjEdge struct {
	to     string
	edge   *graphstructure.Edge
	weight float64
}

func shortestPath(g *graphstructure.Graph, fromID, toID string) ([]string, float64, bool) {
	if fromID == toID {
		return []string{fromID}, 0, true
	}
	adj := map[string][]routeAdjEdge{}
	for _, edge := range g.Edges {
		weight := edge.WeightKm
		if weight <= 0 {
			weight = 0.000001
		}
		adj[edge.From] = append(adj[edge.From], routeAdjEdge{to: edge.To, edge: edge, weight: weight})
		adj[edge.To] = append(adj[edge.To], routeAdjEdge{to: edge.From, edge: edge, weight: weight})
	}

	dist := map[string]float64{fromID: 0}
	prev := map[string]string{}
	pq := &routePriorityQueue{}
	heap.Push(pq, routeQueueItem{id: fromID, dist: 0})
	for pq.Len() > 0 {
		item := heap.Pop(pq).(routeQueueItem)
		if item.dist > dist[item.id] {
			continue
		}
		if item.id == toID {
			break
		}
		for _, next := range adj[item.id] {
			nextDist := item.dist + next.weight
			if current, ok := dist[next.to]; ok && current <= nextDist {
				continue
			}
			dist[next.to] = nextDist
			prev[next.to] = item.id
			heap.Push(pq, routeQueueItem{id: next.to, dist: nextDist})
		}
	}
	total, ok := dist[toID]
	if !ok {
		return nil, 0, false
	}
	path := []string{}
	for id := toID; id != ""; id = prev[id] {
		path = append(path, id)
		if id == fromID {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, total, true
}

func findEdgeBetween(g *graphstructure.Graph, a, b string) *graphstructure.Edge {
	for _, edge := range g.Edges {
		if (edge.From == a && edge.To == b) || (edge.From == b && edge.To == a) {
			return edge
		}
	}
	return nil
}

type routeQueueItem struct {
	id   string
	dist float64
}

type routePriorityQueue []routeQueueItem

func (pq routePriorityQueue) Len() int { return len(pq) }

func (pq routePriorityQueue) Less(i, j int) bool { return pq[i].dist < pq[j].dist }

func (pq routePriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *routePriorityQueue) Push(x any) {
	*pq = append(*pq, x.(routeQueueItem))
}

func (pq *routePriorityQueue) Pop() any {
	old := *pq
	item := old[len(old)-1]
	*pq = old[:len(old)-1]
	return item
}

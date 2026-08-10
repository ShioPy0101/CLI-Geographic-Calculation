package sqlreq

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"CLI-Geographic-Calculation/pkg/giocal"
	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
)

type RouteQuery struct {
	Selections []RouteSelection
	Options    RouteQueryOptions
}

type RouteQueryOptions struct {
	SingleLine     bool `json:"single_line"`
	Geographic     bool `json:"geographic"`
	NoAnimation    bool `json:"no_animation"`
	NoLabels       bool `json:"no_labels"`
	EndpointLabels bool `json:"endpoint_labels"`
}

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
	routeTokenOption
	routeTokenSingleLine
	routeTokenGeographic
	routeTokenNoAnimation
	routeTokenNoLabels
	routeTokenEndpointLabels
	routeTokenComma
	routeTokenSemicolon
)

type routeToken struct {
	kind routeTokenKind
	text string
	pos  int
}

func ParseRouteSelectionQuery(query string) ([]RouteSelection, error) {
	routeQuery, err := ParseRouteQuery(query)
	if err != nil {
		return nil, err
	}
	return routeQuery.Selections, nil
}

func ParseRouteQuery(query string) (RouteQuery, error) {
	tokens, err := lexRouteSelectionQuery(query)
	if err != nil {
		return RouteQuery{}, err
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
		case "OPTION":
			tokens = append(tokens, routeToken{kind: routeTokenOption, text: text, pos: start})
		case "SINGLE_LINE", "SINGLE-LINE":
			tokens = append(tokens, routeToken{kind: routeTokenSingleLine, text: text, pos: start})
		case "GEOGRAPHIC":
			tokens = append(tokens, routeToken{kind: routeTokenGeographic, text: text, pos: start})
		case "NO_ANIMATION", "NO-ANIMATION", "STATIC":
			tokens = append(tokens, routeToken{kind: routeTokenNoAnimation, text: text, pos: start})
		case "NO_LABELS", "NO-LABELS":
			tokens = append(tokens, routeToken{kind: routeTokenNoLabels, text: text, pos: start})
		case "ENDPOINT_LABELS", "ENDPOINT-LABELS":
			tokens = append(tokens, routeToken{kind: routeTokenEndpointLabels, text: text, pos: start})
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

func (p *routeSelectionParser) parse() (RouteQuery, error) {
	if _, err := p.expect(routeTokenSelect); err != nil {
		return RouteQuery{}, err
	}
	selections := []RouteSelection{}
	for {
		selection, err := p.parseSelection()
		if err != nil {
			return RouteQuery{}, err
		}
		selections = append(selections, selection)

		if p.match(routeTokenComma) {
			continue
		}
		break
	}
	options := RouteQueryOptions{}
	if p.match(routeTokenOption) {
		for {
			if p.match(routeTokenSingleLine) {
				options.SingleLine = true
			} else if p.match(routeTokenGeographic) {
				options.Geographic = true
			} else if p.match(routeTokenNoAnimation) {
				options.NoAnimation = true
			} else if p.match(routeTokenNoLabels) {
				options.NoLabels = true
			} else if p.match(routeTokenEndpointLabels) {
				options.EndpointLabels = true
			} else {
				return RouteQuery{}, fmt.Errorf("expected route option at byte %d", p.peek().pos)
			}
			if !p.match(routeTokenComma) {
				break
			}
		}
	}
	p.match(routeTokenSemicolon)
	if p.peek().kind != routeTokenEOF {
		return RouteQuery{}, fmt.Errorf("unexpected token %q at byte %d", p.peek().text, p.peek().pos)
	}
	return RouteQuery{Selections: selections, Options: options}, nil
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
	case routeTokenOption:
		return "OPTION"
	case routeTokenSingleLine:
		return "single_line"
	case routeTokenGeographic:
		return "geographic"
	case routeTokenNoAnimation:
		return "no_animation"
	case routeTokenNoLabels:
		return "no_labels"
	case routeTokenEndpointLabels:
		return "endpoint_labels"
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
		parts, err := resolveRouteSelectionParts(selection, drs)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			mergeGraph(out, part.Graph)
		}
	}
	return out, nil
}

func SQLLikeToResolvedRoute(query string, drs *giocaltype.DatasetResource) (routepath.ResolvedRoute, RouteQueryOptions, error) {
	routeQuery, err := ParseRouteQuery(query)
	if err != nil {
		return routepath.ResolvedRoute{}, RouteQueryOptions{}, err
	}
	resolved, err := RouteSelectionsToResolvedRoute(routeQuery.Selections, drs)
	if err != nil {
		return routepath.ResolvedRoute{}, RouteQueryOptions{}, err
	}
	return resolved, routeQuery.Options, nil
}

func RouteSelectionsToResolvedRoute(selections []RouteSelection, drs *giocaltype.DatasetResource) (routepath.ResolvedRoute, error) {
	if len(selections) == 0 {
		return routepath.ResolvedRoute{}, errors.New("at least one route selection is required")
	}
	segments := make([]routepath.RouteSegment, 0, len(selections))
	for _, selection := range selections {
		parts, err := resolveRouteSelectionParts(selection, drs)
		if err != nil {
			return routepath.ResolvedRoute{}, err
		}
		for _, part := range parts {
			segments = append(segments, part.Segment)
		}
	}
	return routepath.ResolvedRoute{Segments: segments}, nil
}

type resolvedRoutePart struct {
	Graph   *graphstructure.Graph
	Segment routepath.RouteSegment
}

func resolveRouteSelectionParts(selection RouteSelection, drs *giocaltype.DatasetResource) ([]resolvedRoutePart, error) {
	graph, segment, err := resolveRouteSelection(selection, drs)
	if err == nil {
		return []resolvedRoutePart{{Graph: graph, Segment: segment}}, nil
	}
	if !selection.HasStationRange() || selection.FromStation == nil || selection.ToStation == nil {
		return nil, err
	}
	parts, splitErr := resolveRouteSelectionViaAdjacentLine(selection, drs)
	if splitErr == nil {
		return parts, nil
	}
	return nil, err
}

func resolveRouteSelection(selection RouteSelection, drs *giocaltype.DatasetResource) (*graphstructure.Graph, routepath.RouteSegment, error) {
	if selection.Line == "" {
		return nil, routepath.RouteSegment{}, errors.New("line name is required")
	}
	lineRailIndices := railIndicesForLine(drs, selection.Line)
	if len(lineRailIndices) == 0 {
		return nil, routepath.RouteSegment{}, fmt.Errorf("line %q does not exist", selection.Line)
	}
	lineStationIndices := stationIndicesForLine(drs, selection.Line)
	lineGraph := giocal.ConvertGiotypeRailwayToGraphByRequired(drs.Station, drs.Rail, lineStationIndices, lineRailIndices)
	if !selection.HasStationRange() {
		pathIDs := coordPathIDsForLine(drs, lineRailIndices)
		return lineGraph, routepath.RouteSegment{LineID: selection.Line, Stations: orderedStationsForPath(lineGraph, selection.Line, pathIDs)}, nil
	}
	if selection.FromStation == nil || selection.ToStation == nil {
		return nil, routepath.RouteSegment{}, fmt.Errorf("line %q must specify both from_station and to_station", selection.Line)
	}
	if !stationExists(drs, *selection.FromStation) {
		return nil, routepath.RouteSegment{}, fmt.Errorf("from station %q does not exist", *selection.FromStation)
	}
	if !stationExists(drs, *selection.ToStation) {
		return nil, routepath.RouteSegment{}, fmt.Errorf("to station %q does not exist", *selection.ToStation)
	}
	if !stationExistsOnLine(drs, selection.Line, *selection.FromStation) {
		return nil, routepath.RouteSegment{}, fmt.Errorf("from station %q does not exist on line %q", *selection.FromStation, selection.Line)
	}
	if !stationExistsOnLine(drs, selection.Line, *selection.ToStation) {
		return nil, routepath.RouteSegment{}, fmt.Errorf("to station %q does not exist on line %q", *selection.ToStation, selection.Line)
	}

	sectionGraph, pathIDs, err := extractStationRangeGraph(lineGraph, selection.Line, *selection.FromStation, *selection.ToStation)
	if err != nil {
		return nil, routepath.RouteSegment{}, err
	}
	return sectionGraph, routepath.RouteSegment{LineID: selection.Line, Stations: orderedStationsForPath(sectionGraph, selection.Line, pathIDs)}, nil
}

func resolveRouteSelectionViaAdjacentLine(selection RouteSelection, drs *giocaltype.DatasetResource) ([]resolvedRoutePart, error) {
	if selection.Line == "" {
		return nil, errors.New("line name is required")
	}
	if selection.FromStation == nil || selection.ToStation == nil {
		return nil, fmt.Errorf("line %q must specify both from_station and to_station", selection.Line)
	}
	fromStation := *selection.FromStation
	toStation := *selection.ToStation
	if !stationExists(drs, fromStation) {
		return nil, fmt.Errorf("from station %q does not exist", fromStation)
	}
	if !stationExists(drs, toStation) {
		return nil, fmt.Errorf("to station %q does not exist", toStation)
	}

	fromOnSelectedLine := stationExistsOnLine(drs, selection.Line, fromStation)
	toOnSelectedLine := stationExistsOnLine(drs, selection.Line, toStation)
	if fromOnSelectedLine == toOnSelectedLine {
		return nil, fmt.Errorf("route %q between %q and %q cannot be split through an adjacent line", selection.Line, fromStation, toStation)
	}

	selectedEndpoint := fromStation
	remoteEndpoint := toStation
	selectedFirst := true
	if !fromOnSelectedLine {
		selectedEndpoint = toStation
		remoteEndpoint = fromStation
		selectedFirst = false
	}

	bestParts := []resolvedRoutePart(nil)
	bestCost := routeInf
	for _, candidateLine := range sortedLinesForStation(drs, remoteEndpoint) {
		if candidateLine == selection.Line {
			continue
		}
		for _, transferStation := range sortedSharedStations(drs, selection.Line, candidateLine) {
			firstSelection := routeSelectionBetween(selection.Line, selectedEndpoint, transferStation)
			secondSelection := routeSelectionBetween(candidateLine, transferStation, remoteEndpoint)
			if !selectedFirst {
				firstSelection = routeSelectionBetween(candidateLine, remoteEndpoint, transferStation)
				secondSelection = routeSelectionBetween(selection.Line, transferStation, selectedEndpoint)
			}
			firstGraph, firstSegment, err := resolveRouteSelection(firstSelection, drs)
			if err != nil {
				continue
			}
			secondGraph, secondSegment, err := resolveRouteSelection(secondSelection, drs)
			if err != nil {
				continue
			}
			cost := routeSegmentDistanceKm(firstSegment) + routeSegmentDistanceKm(secondSegment)
			if cost < bestCost {
				bestCost = cost
				bestParts = []resolvedRoutePart{
					{Graph: firstGraph, Segment: firstSegment},
					{Graph: secondGraph, Segment: secondSegment},
				}
			}
		}
	}
	if len(bestParts) == 0 {
		return nil, fmt.Errorf("no adjacent line route found for line %q between %q and %q", selection.Line, fromStation, toStation)
	}
	return bestParts, nil
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

func sortedLinesForStation(drs *giocaltype.DatasetResource, stationName string) []string {
	seen := map[string]bool{}
	for _, station := range drs.Station.Features {
		if station.Properties.N02005 == stationName && station.Properties.N02003 != "" {
			seen[station.Properties.N02003] = true
		}
	}
	return sortedMapKeys(seen)
}

func sortedSharedStations(drs *giocaltype.DatasetResource, lineA, lineB string) []string {
	shared := map[string]bool{}
	for _, stationA := range drs.Station.Features {
		if stationA.Properties.N02003 != lineA {
			continue
		}
		for _, stationB := range drs.Station.Features {
			if stationB.Properties.N02003 == lineB && sameStationFeature(stationA, stationB) {
				shared[stationA.Properties.N02005] = true
			}
		}
	}
	return sortedMapKeys(shared)
}

func sameStationFeature(a, b giocaltype.GiotypeStation) bool {
	if a.Properties.N02005 == "" || a.Properties.N02005 != b.Properties.N02005 {
		return false
	}
	if a.Properties.N02005g != "" && b.Properties.N02005g != "" {
		return a.Properties.N02005g == b.Properties.N02005g
	}
	aLon, aLat, aOK := stationFeaturePoint(a)
	bLon, bLat, bOK := stationFeaturePoint(b)
	if !aOK || !bOK {
		return false
	}
	return stationFeatureDistanceKm(aLon, aLat, bLon, bLat) <= 0.3
}

func stationFeaturePoint(station giocaltype.GiotypeStation) (lon, lat float64, ok bool) {
	for _, pair := range station.Geometry.Coordinates {
		if len(pair) >= 2 {
			return pair[0], pair[1], true
		}
	}
	return 0, 0, false
}

func stationFeatureDistanceKm(lonA, latA, lonB, latB float64) float64 {
	dxKm := (lonB - lonA) / giocal.LongitudePerKm
	dyKm := (latB - latA) / giocal.LatitudePerKm
	return math.Sqrt(dxKm*dxKm + dyKm*dyKm)
}

func sortedMapKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func routeSelectionBetween(line, fromStation, toStation string) RouteSelection {
	from := fromStation
	to := toStation
	return RouteSelection{Line: line, FromStation: &from, ToStation: &to}
}

func routeSegmentDistanceKm(segment routepath.RouteSegment) float64 {
	total := 0.0
	for i := 1; i < len(segment.Stations); i++ {
		prev := segment.Stations[i-1]
		next := segment.Stations[i]
		if prev == nil || next == nil {
			continue
		}
		total += stationDistanceKm(prev, next)
	}
	return total
}

func stationDistanceKm(a, b *graphstructure.Node) float64 {
	dxKm := (b.Lon - a.Lon) / giocal.LongitudePerKm
	dyKm := (b.Lat - a.Lat) / giocal.LatitudePerKm
	return math.Sqrt(dxKm*dxKm + dyKm*dyKm)
}

func extractStationRangeGraph(g *graphstructure.Graph, line, fromStation, toStation string) (*graphstructure.Graph, []string, error) {
	fromIDs := stationNodeIDs(g, line, fromStation)
	toIDs := stationNodeIDs(g, line, toStation)
	if len(fromIDs) == 0 {
		return nil, nil, fmt.Errorf("from station %q was not found in graph for line %q", fromStation, line)
	}
	if len(toIDs) == 0 {
		return nil, nil, fmt.Errorf("to station %q was not found in graph for line %q", toStation, line)
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
		return nil, nil, fmt.Errorf("no path found on line %q between %q and %q", line, fromStation, toStation)
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
	return out, bestPath, nil
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

func coordPathIDsForLine(drs *giocaltype.DatasetResource, railIndices []int) []string {
	out := []string{}
	for _, index := range railIndices {
		if index < 0 || index >= len(drs.Rail.Features) {
			continue
		}
		for _, pair := range drs.Rail.Features[index].Geometry.Coordinates {
			if len(pair) < 2 {
				continue
			}
			id := coordNodeID(pair[0], pair[1])
			if len(out) > 0 && out[len(out)-1] == id {
				continue
			}
			out = append(out, id)
		}
	}
	return out
}

func orderedStationsForPath(g *graphstructure.Graph, line string, pathIDs []string) []*graphstructure.Node {
	stationsByCoord := map[string][]*graphstructure.Node{}
	for _, edge := range g.Edges {
		if edge.Kind != "station_at" || edge.Meta["line"] != line {
			continue
		}
		from := g.Nodes[edge.From]
		to := g.Nodes[edge.To]
		if from == nil || to == nil {
			continue
		}
		if from.Kind == "station" && to.Kind == "coord" {
			stationsByCoord[to.ID] = append(stationsByCoord[to.ID], from)
		} else if from.Kind == "coord" && to.Kind == "station" {
			stationsByCoord[from.ID] = append(stationsByCoord[from.ID], to)
		}
	}

	out := []*graphstructure.Node{}
	for _, id := range pathIDs {
		node := g.Nodes[id]
		if node != nil && node.Kind == "station" && node.Meta["line"] == line {
			out = appendStationIfNewBoundary(out, node)
			continue
		}
		for _, station := range stationsByCoord[id] {
			out = appendStationIfNewBoundary(out, station)
		}
	}
	return out
}

func appendStationIfNewBoundary(stations []*graphstructure.Node, station *graphstructure.Node) []*graphstructure.Node {
	if station == nil {
		return stations
	}
	if len(stations) > 0 && stations[len(stations)-1].ID == station.ID {
		return stations
	}
	return append(stations, station)
}

func coordNodeID(lon, lat float64) string {
	return fmt.Sprintf("coord:%.6f,%.6f", lon, lat)
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

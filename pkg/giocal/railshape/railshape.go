package railshape

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
)

const (
	LineProperty     = "N05_002"
	OperatorProperty = "N05_003"
	SectionProperty  = "N05_006"
	bridgeTolerance  = 0.03
)

type GeoJSON struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   Geometry       `json:"geometry"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates coordinates `json:"coordinates"`
}

type coordinates [][]float64

type Point struct {
	Lon float64
	Lat float64
}

type PathGeometry struct {
	Points []Point
}

type Resolver struct {
	featuresByLine map[string][]Feature
}

func Load(path string) (*Resolver, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc GeoJSON
	if err := json.Unmarshal(b, &fc); err != nil {
		return nil, err
	}
	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("railroad shape geojson must be FeatureCollection, got %q", fc.Type)
	}
	r := &Resolver{featuresByLine: map[string][]Feature{}}
	for _, feature := range fc.Features {
		if feature.Geometry.Type != "LineString" {
			continue
		}
		line := propertyString(feature.Properties, LineProperty)
		if line == "" {
			continue
		}
		r.featuresByLine[line] = append(r.featuresByLine[line], feature)
	}
	return r, nil
}

func (r *Resolver) Resolve(route routepath.ResolvedRoute) ([]PathGeometry, error) {
	out := make([]PathGeometry, 0, len(route.Segments))
	for i, segment := range route.Segments {
		geometry, err := r.ResolveSegment(segment)
		if err != nil {
			return nil, fmt.Errorf("segment %d: %w", i+1, err)
		}
		if len(geometry.Points) > 0 {
			if len(out) > 0 {
				bridgeContinuousSegments(&out[len(out)-1], &geometry, route.Segments[i-1], segment)
			}
			out = append(out, geometry)
		}
	}
	return out, nil
}

func bridgeContinuousSegments(prev, next *PathGeometry, prevSegment, nextSegment routepath.RouteSegment) {
	if prev == nil || next == nil || len(prev.Points) == 0 || len(next.Points) == 0 {
		return
	}
	station := sharedBoundaryStation(prevSegment, nextSegment)
	if station == nil {
		return
	}
	bridge := pointFromNode(station)
	prevEnd := prev.Points[len(prev.Points)-1]
	nextStart := next.Points[0]
	nextEnd := next.Points[len(next.Points)-1]
	if connectablePoint(prevEnd, nextStart) || connectablePoint(prevEnd, nextEnd) {
		return
	}
	if projectedDistance(prevEnd, bridge) <= bridgeTolerance && projectedDistance(nextStart, bridge) <= bridgeTolerance {
		prev.Points = appendPointIfDifferent(prev.Points, bridge)
		next.Points = prependPointIfDifferent(next.Points, bridge)
		return
	}
	if projectedDistance(prevEnd, bridge) <= bridgeTolerance && projectedDistance(nextEnd, bridge) <= bridgeTolerance {
		reversePoints(next.Points)
		prev.Points = appendPointIfDifferent(prev.Points, bridge)
		next.Points = prependPointIfDifferent(next.Points, bridge)
	}
}

func sharedBoundaryStation(prev, next routepath.RouteSegment) *graphstructure.Node {
	if len(prev.Stations) == 0 || len(next.Stations) == 0 {
		return nil
	}
	prevEnd := prev.Stations[len(prev.Stations)-1]
	nextStart := next.Stations[0]
	if sameStationNode(prevEnd, nextStart) {
		return prevEnd
	}
	nextEnd := next.Stations[len(next.Stations)-1]
	if sameStationNode(prevEnd, nextEnd) {
		return prevEnd
	}
	return nil
}

func FlattenContinuous(paths []PathGeometry) (PathGeometry, error) {
	if len(paths) == 0 {
		return PathGeometry{}, fmt.Errorf("geographic mode requires at least one shape")
	}
	out := PathGeometry{Points: clonePoints(paths[0].Points)}
	for i := 1; i < len(paths); i++ {
		next := clonePoints(paths[i].Points)
		if len(next) == 0 {
			return PathGeometry{}, fmt.Errorf("geographic mode requires continuous shapes: segment %d is empty", i+1)
		}
		if connectablePoint(out.Points[len(out.Points)-1], next[0]) {
			out.Points = append(out.Points, next[1:]...)
			continue
		}
		if connectablePoint(out.Points[len(out.Points)-1], next[len(next)-1]) {
			reversePoints(next)
			out.Points = append(out.Points, next[1:]...)
			continue
		}
		return PathGeometry{}, fmt.Errorf("geographic mode requires continuous shapes: segment %d does not connect to the current shape", i+1)
	}
	return out, nil
}

func (r *Resolver) ResolveSegment(segment routepath.RouteSegment) (PathGeometry, error) {
	if segment.LineID == "" {
		return PathGeometry{}, fmt.Errorf("line name is required")
	}
	features := r.featuresForSegment(segment)
	if len(features) == 0 {
		return PathGeometry{}, fmt.Errorf("line %q does not exist in railroad shape geojson", segment.LineID)
	}
	components := mergeFeatures(features)
	if len(components) == 0 {
		return PathGeometry{}, fmt.Errorf("line %q has no shape coordinates", segment.LineID)
	}
	if len(segment.Stations) == 0 {
		return PathGeometry{Points: components[0]}, nil
	}
	start := segment.Stations[0]
	end := segment.Stations[len(segment.Stations)-1]
	if start == nil || end == nil {
		return PathGeometry{}, fmt.Errorf("line %q has invalid route endpoints", segment.LineID)
	}

	best := PathGeometry{}
	bestScore := math.Inf(1)
	for _, component := range components {
		startPos := nearestPosition(component, pointFromNode(start))
		endPos := nearestPosition(component, pointFromNode(end))
		if !startPos.Valid || !endPos.Valid {
			continue
		}
		score := startPos.Distance + endPos.Distance
		if score >= bestScore {
			continue
		}
		sliced := slicePolyline(component, startPos, endPos)
		if len(sliced) == 0 {
			continue
		}
		best = PathGeometry{Points: sliced}
		bestScore = score
	}
	if len(best.Points) == 0 {
		return PathGeometry{}, fmt.Errorf("line %q shape could not be matched to route endpoints", segment.LineID)
	}
	return best, nil
}

func (r *Resolver) featuresForSegment(segment routepath.RouteSegment) []Feature {
	features := r.featuresByLine[segment.LineID]
	company := segmentCompany(segment)
	if company == "" {
		return features
	}
	filtered := []Feature{}
	for _, feature := range features {
		if sameOperator(propertyString(feature.Properties, OperatorProperty), company) {
			filtered = append(filtered, feature)
		}
	}
	if len(filtered) == 0 {
		return features
	}
	return filtered
}

func segmentCompany(segment routepath.RouteSegment) string {
	for _, station := range segment.Stations {
		if station != nil && station.Meta != nil && station.Meta["company"] != "" {
			return station.Meta["company"]
		}
	}
	return ""
}

func sameOperator(shapeOperator, stationCompany string) bool {
	return normalizeOperator(shapeOperator) == normalizeOperator(stationCompany)
}

func normalizeOperator(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "（旧国鉄）", "")
	return s
}

func propertyString(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	value := props[key]
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func mergeFeatures(features []Feature) [][]Point {
	components := [][]Point{}
	for _, feature := range features {
		points := pointsFromFeature(feature)
		if len(points) == 0 {
			continue
		}
		components = append(components, points)
	}

	changed := true
	for changed {
		changed = false
		for i := 0; i < len(components) && !changed; i++ {
			for j := i + 1; j < len(components); j++ {
				merged, ok := mergeTwo(components[i], components[j])
				if !ok {
					continue
				}
				components[i] = merged
				components = append(components[:j], components[j+1:]...)
				changed = true
				break
			}
		}
	}
	return components
}

func pointsFromFeature(feature Feature) []Point {
	out := make([]Point, 0, len(feature.Geometry.Coordinates))
	for _, pair := range feature.Geometry.Coordinates {
		if len(pair) < 2 {
			continue
		}
		out = append(out, Point{Lon: pair[0], Lat: pair[1]})
	}
	return out
}

func mergeTwo(a, b []Point) ([]Point, bool) {
	if len(a) == 0 || len(b) == 0 {
		return nil, false
	}
	a0, a1 := a[0], a[len(a)-1]
	b0, b1 := b[0], b[len(b)-1]
	switch {
	case samePoint(a1, b0):
		return append(clonePoints(a), b[1:]...), true
	case samePoint(a1, b1):
		rb := clonePoints(b)
		reversePoints(rb)
		return append(clonePoints(a), rb[1:]...), true
	case samePoint(a0, b1):
		return append(clonePoints(b), a[1:]...), true
	case samePoint(a0, b0):
		rb := clonePoints(b)
		reversePoints(rb)
		return append(rb, a[1:]...), true
	default:
		return nil, false
	}
}

type position struct {
	SegmentIndex int
	T            float64
	Point        Point
	Distance     float64
	Measure      float64
	Valid        bool
}

func nearestPosition(polyline []Point, target Point) position {
	if len(polyline) == 0 {
		return position{}
	}
	if len(polyline) == 1 {
		return position{SegmentIndex: 0, T: 0, Point: polyline[0], Distance: projectedDistance(polyline[0], target), Valid: true}
	}
	best := position{Distance: math.Inf(1)}
	measure := 0.0
	for i := 0; i < len(polyline)-1; i++ {
		a, b := polyline[i], polyline[i+1]
		proj, t, dist := nearestOnSegment(a, b, target)
		if dist < best.Distance {
			best = position{SegmentIndex: i, T: t, Point: proj, Distance: dist, Measure: measure + t*projectedDistance(a, b), Valid: true}
		}
		measure += projectedDistance(a, b)
	}
	return best
}

func nearestOnSegment(a, b, target Point) (Point, float64, float64) {
	centerLat := (a.Lat + b.Lat + target.Lat) / 3
	scale := math.Cos(centerLat * math.Pi / 180)
	ax, ay := a.Lon*scale, a.Lat
	bx, by := b.Lon*scale, b.Lat
	tx, ty := target.Lon*scale, target.Lat
	dx, dy := bx-ax, by-ay
	denom := dx*dx + dy*dy
	t := 0.0
	if denom > 0 {
		t = ((tx-ax)*dx + (ty-ay)*dy) / denom
	}
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	point := Point{
		Lon: a.Lon + (b.Lon-a.Lon)*t,
		Lat: a.Lat + (b.Lat-a.Lat)*t,
	}
	return point, t, projectedDistance(point, target)
}

func slicePolyline(polyline []Point, start, end position) []Point {
	if start.Measure > end.Measure {
		out := slicePolylineForward(polyline, end, start)
		reversePoints(out)
		return out
	}
	return slicePolylineForward(polyline, start, end)
}

func slicePolylineForward(polyline []Point, start, end position) []Point {
	if len(polyline) == 0 || !start.Valid || !end.Valid {
		return nil
	}
	if start.SegmentIndex > end.SegmentIndex || (start.SegmentIndex == end.SegmentIndex && start.T > end.T) {
		return nil
	}
	out := []Point{start.Point}
	if start.SegmentIndex == end.SegmentIndex {
		if !samePoint(start.Point, end.Point) {
			out = append(out, end.Point)
		}
		return out
	}
	for i := start.SegmentIndex + 1; i <= end.SegmentIndex; i++ {
		out = appendPointIfDifferent(out, polyline[i])
	}
	out = appendPointIfDifferent(out, end.Point)
	return out
}

func pointFromNode(node *graphstructure.Node) Point {
	return Point{Lon: node.Lon, Lat: node.Lat}
}

func projectedDistance(a, b Point) float64 {
	centerLat := (a.Lat + b.Lat) / 2
	scale := math.Cos(centerLat * math.Pi / 180)
	dx := (a.Lon - b.Lon) * scale
	dy := a.Lat - b.Lat
	return math.Sqrt(dx*dx + dy*dy)
}

func appendPointIfDifferent(points []Point, point Point) []Point {
	if len(points) > 0 && samePoint(points[len(points)-1], point) {
		return points
	}
	return append(points, point)
}

func prependPointIfDifferent(points []Point, point Point) []Point {
	if len(points) > 0 && samePoint(points[0], point) {
		return points
	}
	out := make([]Point, 0, len(points)+1)
	out = append(out, point)
	out = append(out, points...)
	return out
}

func sameStationNode(a, b *graphstructure.Node) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != "" && b.ID != "" && a.ID == b.ID {
		return true
	}
	return a.Name != "" && a.Name == b.Name
}

func clonePoints(points []Point) []Point {
	out := make([]Point, len(points))
	copy(out, points)
	return out
}

func reversePoints(points []Point) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

func samePoint(a, b Point) bool {
	return math.Abs(a.Lon-b.Lon) <= 1e-7 && math.Abs(a.Lat-b.Lat) <= 1e-7
}

func connectablePoint(a, b Point) bool {
	return samePoint(a, b) || projectedDistance(a, b) <= 0.01
}

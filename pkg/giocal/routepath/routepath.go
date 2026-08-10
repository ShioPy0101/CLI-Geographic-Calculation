package routepath

import (
	"fmt"
	"math"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
)

type ResolvedRoute struct {
	Segments []RouteSegment
}

type RouteSegment struct {
	LineID   string
	Stations []*graphstructure.Node
}

type RenderPath struct {
	Stations []*graphstructure.Node
}

type FlattenOptions struct {
	AllowReverse bool
}

func RenderPathsFromResolvedRoute(route ResolvedRoute) []RenderPath {
	out := make([]RenderPath, 0, len(route.Segments))
	for _, segment := range route.Segments {
		if len(segment.Stations) == 0 {
			continue
		}
		out = append(out, RenderPath{Stations: cloneStations(segment.Stations)})
	}
	return out
}

func FlattenContinuousRoute(route ResolvedRoute, opt FlattenOptions) (RenderPath, error) {
	if len(route.Segments) == 0 {
		return RenderPath{}, fmt.Errorf("single-line mode requires a continuous route: no segments")
	}

	var stations []*graphstructure.Node
	for i, segment := range route.Segments {
		next := cloneStations(segment.Stations)
		if len(next) == 0 {
			return RenderPath{}, fmt.Errorf("single-line mode requires a continuous route: segment %d is empty", i+1)
		}
		if len(stations) == 0 {
			stations = append(stations, next...)
			continue
		}

		currentEnd := stations[len(stations)-1]
		switch {
		case sameStation(currentEnd, next[0]):
			stations = append(stations, next[1:]...)
		case opt.AllowReverse && sameStation(currentEnd, next[len(next)-1]):
			reverseStations(next)
			stations = append(stations, next[1:]...)
		default:
			return RenderPath{}, fmt.Errorf("single-line mode requires a continuous route: segment %d does not connect to the current route", i+1)
		}
	}
	return RenderPath{Stations: stations}, nil
}

func cloneStations(stations []*graphstructure.Node) []*graphstructure.Node {
	out := make([]*graphstructure.Node, len(stations))
	copy(out, stations)
	return out
}

func reverseStations(stations []*graphstructure.Node) {
	for i, j := 0, len(stations)-1; i < j; i, j = i+1, j-1 {
		stations[i], stations[j] = stations[j], stations[i]
	}
}

func sameStation(a, b *graphstructure.Node) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != "" && b.ID != "" && a.ID == b.ID {
		return true
	}
	if a.Name == "" || a.Name != b.Name {
		return false
	}
	return nearlySameCoordinate(a.Lon, b.Lon) && nearlySameCoordinate(a.Lat, b.Lat)
}

func nearlySameCoordinate(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

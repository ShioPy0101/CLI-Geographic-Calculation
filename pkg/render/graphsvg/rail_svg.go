package graphsvg

import (
	"bytes"
	"fmt"
	"html"
	"sort"
	"strings"

	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/railshape"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
	"CLI-Geographic-Calculation/pkg/render/geo"
)

const labelFontFamily = `"Noto Sans CJK JP","Noto Sans JP","Hiragino Sans","Yu Gothic","Meiryo",sans-serif`

type Options struct {
	Width        int
	Height       int
	Padding      int
	DrawStations bool
	DrawLabels   bool
	AnimatePath  bool
}

func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 1200
	}
	if o.Height <= 0 {
		o.Height = 800
	}
	if o.Padding < 0 {
		o.Padding = 0
	}
	return o
}

// RenderRailGraphSVG: rail(graph.Edges Kind="rail") を座標として描画する簡易 SVG
func RenderRailGraphSVG(graph any, opt Options) (string, error) {
	opt = opt.withDefaults()

	g, ok := graph.(*graphstructure.Graph)
	if !ok {
		return "", fmt.Errorf("unexpected graph type: %T (expected *graphstructure.Graph)", graph)
	}

	// 座標ノード収集（rail のエッジに登場する coord ノードを優先）
	coordIDs := collectCoordIDsFromRailEdges(g)

	// fallback: 何も取れなければ Nodes の coord 全部
	if len(coordIDs) == 0 {
		for id, n := range g.Nodes {
			if n.Kind == "coord" {
				coordIDs = append(coordIDs, id)
			}
		}
	}

	if len(coordIDs) == 0 {
		return "", fmt.Errorf("no coord nodes found")
	}

	points := make([]geo.GeographicPoint, 0, len(coordIDs))
	for _, id := range coordIDs {
		n, ok := g.Nodes[id]
		if !ok {
			continue
		}
		points = append(points, geo.GeographicPoint{ID: id, Lon: n.Lon, Lat: n.Lat})
	}
	for _, s := range collectStations(g) {
		points = append(points, geo.GeographicPoint{ID: s.ID, Lon: s.Lon, Lat: s.Lat})
	}

	screenByID := screenPointsByID(points, opt)

	// rail edge をセグメントとして描画
	railEdges := make([]graphstructure.Edge, 0, len(g.Edges))
	for _, e := range g.Edges {
		if e.Kind == "rail" {
			railEdges = append(railEdges, *e)
		}
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		opt.Width, opt.Height, opt.Width, opt.Height))
	buf.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="white"/>` + "\n")

	// 線（rail）
	buf.WriteString(`<g fill="none" stroke="#111" stroke-width="1" stroke-linecap="round" stroke-linejoin="round" opacity="0.9">` + "\n")
	for _, e := range railEdges {
		a, okA := g.Nodes[e.From]
		b, okB := g.Nodes[e.To]
		if !okA || !okB {
			continue
		}
		if a.Kind != "coord" || b.Kind != "coord" {
			continue
		}
		p1, ok1 := screenByID[a.ID]
		p2, ok2 := screenByID[b.ID]
		if !ok1 || !ok2 {
			continue
		}
		buf.WriteString(fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f"/>`+"\n", p1.X, p1.Y, p2.X, p2.Y))
	}
	buf.WriteString(`</g>` + "\n")

	// station
	if opt.DrawStations {
		stations := collectStations(g)

		// 駅を上に描く
		buf.WriteString(`<g>` + "\n")
		for _, s := range stations {
			screen, ok := screenByID[s.ID]
			if !ok {
				continue
			}
			x, y := screen.X, screen.Y
			buf.WriteString(fmt.Sprintf(`<circle cx="%.2f" cy="%.2f" r="3" fill="#d00" stroke="#fff" stroke-width="1"/>`+"\n", x, y))
			if opt.DrawLabels && s.Name != "" {
				// 文字が被るので右上に少しずらす
				label := html.EscapeString(s.Name)
				buf.WriteString(fmt.Sprintf(`<text x="%.2f" y="%.2f" font-family='%s' font-size="12" fill="none" stroke="white" stroke-width="3">%s</text>`+"\n",
					x+6, y-6, labelFontFamily, label))
				buf.WriteString(fmt.Sprintf(`<text x="%.2f" y="%.2f" font-family='%s' font-size="12" fill="#111">%s</text>`+"\n",
					x+6, y-6, labelFontFamily, label))
			}
		}
		buf.WriteString(`</g>` + "\n")
	}

	buf.WriteString(`</svg>` + "\n")
	return buf.String(), nil
}

func RenderRailPathsSVG(paths []routepath.RenderPath, opt Options) (string, error) {
	opt = opt.withDefaults()
	points := geographicPointsFromRenderPaths(paths)
	if len(points) == 0 {
		return "", fmt.Errorf("no stations found")
	}
	screenByID := screenPointsByID(points, opt)

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		opt.Width, opt.Height, opt.Width, opt.Height))
	buf.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="white"/>` + "\n")

	buf.WriteString(`<g fill="none" stroke="#111" stroke-width="1" stroke-linecap="round" stroke-linejoin="round" opacity="0.9">` + "\n")
	for _, path := range paths {
		d := svgPathData(path, screenByID)
		if d == "" {
			continue
		}
		buf.WriteString(fmt.Sprintf(`<path d="%s"/>`+"\n", d))
	}
	buf.WriteString(`</g>` + "\n")

	if opt.DrawStations {
		buf.WriteString(`<g>` + "\n")
		for _, station := range uniqueStationsFromRenderPaths(paths) {
			point, ok := screenByID[station.ID]
			if !ok {
				continue
			}
			buf.WriteString(fmt.Sprintf(`<circle cx="%.2f" cy="%.2f" r="3" fill="#d00" stroke="#fff" stroke-width="1"/>`+"\n", point.X, point.Y))
			if opt.DrawLabels && station.Name != "" {
				label := html.EscapeString(station.Name)
				buf.WriteString(fmt.Sprintf(`<text x="%.2f" y="%.2f" font-family='%s' font-size="12" fill="none" stroke="white" stroke-width="3">%s</text>`+"\n",
					point.X+6, point.Y-6, labelFontFamily, label))
				buf.WriteString(fmt.Sprintf(`<text x="%.2f" y="%.2f" font-family='%s' font-size="12" fill="#111">%s</text>`+"\n",
					point.X+6, point.Y-6, labelFontFamily, label))
			}
		}
		buf.WriteString(`</g>` + "\n")
	}

	buf.WriteString(`</svg>` + "\n")
	return buf.String(), nil
}

func RenderRailGeometriesSVG(paths []railshape.PathGeometry, stations []*graphstructure.Node, opt Options) (string, error) {
	opt = opt.withDefaults()
	points := geographicPointsFromGeometries(paths)
	for _, station := range stations {
		if station != nil {
			points = append(points, geo.GeographicPoint{ID: station.ID, Lon: station.Lon, Lat: station.Lat})
		}
	}
	if len(points) == 0 {
		return "", fmt.Errorf("no geometry points found")
	}
	projected := geo.ProjectStationCoordinates(points)
	projectedByID := map[string]geo.ProjectedPoint{}
	minX, minY, maxX, maxY := 0.0, 0.0, 0.0, 0.0
	for i, point := range projected {
		x, y := point.X, -point.Y
		projectedByID[point.ID] = geo.ProjectedPoint{ID: point.ID, X: x, Y: y}
		if i == 0 {
			minX, maxX, minY, maxY = x, x, y, y
			continue
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	pad := fitContentPadding(maxX-minX, maxY-minY)
	viewBox := fmt.Sprintf("%.6f %.6f %.6f %.6f", minX-pad, minY-pad, maxX-minX+pad*2, maxY-minY+pad*2)

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="%s" preserveAspectRatio="xMidYMid meet">`+"\n",
		opt.Width, opt.Height, viewBox))
	if opt.AnimatePath {
		buf.WriteString(`<style>.rail-path{pathLength:1;stroke-dasharray:1;stroke-dashoffset:1;animation:draw-route 8s linear forwards}@keyframes draw-route{to{stroke-dashoffset:0}}</style>` + "\n")
	}
	buf.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="white"/>` + "\n")
	buf.WriteString(`<g fill="none" stroke="#111" stroke-width="0.003" stroke-linecap="round" stroke-linejoin="round" opacity="0.9">` + "\n")
	for i, path := range paths {
		d := geographicPathData(path, i, projectedByID)
		if d != "" {
			if opt.AnimatePath {
				buf.WriteString(fmt.Sprintf(`<path class="rail-path" pathLength="1" d="%s"/>`+"\n", d))
			} else {
				buf.WriteString(fmt.Sprintf(`<path d="%s"/>`+"\n", d))
			}
		}
	}
	buf.WriteString(`</g>` + "\n")

	if opt.DrawStations {
		buf.WriteString(`<g>` + "\n")
		r := fitContentPadding(maxX-minX, maxY-minY) * 0.12
		if r <= 0 {
			r = 0.002
		}
		for _, station := range uniqueStationNodes(stations) {
			point, ok := projectedByID[station.ID]
			if !ok {
				continue
			}
			buf.WriteString(fmt.Sprintf(`<circle cx="%.6f" cy="%.6f" r="%.6f" fill="#d00" stroke="#fff" stroke-width="%.6f"/>`+"\n", point.X, point.Y, r, r*0.35))
			if opt.DrawLabels && station.Name != "" {
				label := html.EscapeString(station.Name)
				buf.WriteString(fmt.Sprintf(`<text x="%.6f" y="%.6f" font-family='%s' font-size="%.6f" fill="none" stroke="white" stroke-width="%.6f">%s</text>`+"\n",
					point.X+r*2, point.Y-r*2, labelFontFamily, r*4, r, label))
				buf.WriteString(fmt.Sprintf(`<text x="%.6f" y="%.6f" font-family='%s' font-size="%.6f" fill="#111">%s</text>`+"\n",
					point.X+r*2, point.Y-r*2, labelFontFamily, r*4, label))
			}
		}
		buf.WriteString(`</g>` + "\n")
	}

	buf.WriteString(`</svg>` + "\n")
	return buf.String(), nil
}

func screenPointsByID(points []geo.GeographicPoint, opt Options) map[string]geo.ScreenPoint {
	projected := geo.ProjectStationCoordinates(points)
	transform := geo.CalculateViewportTransform(projected, float64(opt.Width), float64(opt.Height), float64(opt.Padding))
	screenPoints := geo.ApplyViewportTransformToAll(projected, transform)
	out := map[string]geo.ScreenPoint{}
	for _, p := range screenPoints {
		out[p.ID] = p
	}
	return out
}

func geographicPointsFromRenderPaths(paths []routepath.RenderPath) []geo.GeographicPoint {
	out := []geo.GeographicPoint{}
	seen := map[string]bool{}
	for _, path := range paths {
		for _, station := range path.Stations {
			if station == nil || station.ID == "" || seen[station.ID] {
				continue
			}
			seen[station.ID] = true
			out = append(out, geo.GeographicPoint{ID: station.ID, Lon: station.Lon, Lat: station.Lat})
		}
	}
	return out
}

func svgPathData(path routepath.RenderPath, screenByID map[string]geo.ScreenPoint) string {
	parts := []string{}
	for _, station := range path.Stations {
		if station == nil {
			continue
		}
		point, ok := screenByID[station.ID]
		if !ok {
			continue
		}
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("M %.2f %.2f", point.X, point.Y))
		} else {
			parts = append(parts, fmt.Sprintf("L %.2f %.2f", point.X, point.Y))
		}
	}
	return strings.Join(parts, " ")
}

func uniqueStationsFromRenderPaths(paths []routepath.RenderPath) []*graphstructure.Node {
	out := []*graphstructure.Node{}
	seen := map[string]bool{}
	for _, path := range paths {
		for _, station := range path.Stations {
			if station == nil || seen[station.ID] {
				continue
			}
			seen[station.ID] = true
			out = append(out, station)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func geographicPointsFromGeometries(paths []railshape.PathGeometry) []geo.GeographicPoint {
	out := []geo.GeographicPoint{}
	for pathIndex, path := range paths {
		for pointIndex, point := range path.Points {
			out = append(out, geo.GeographicPoint{
				ID:  fmt.Sprintf("shape:%d:%d", pathIndex, pointIndex),
				Lon: point.Lon,
				Lat: point.Lat,
			})
		}
	}
	return out
}

func geographicPathData(path railshape.PathGeometry, pathIndex int, projectedByID map[string]geo.ProjectedPoint) string {
	parts := []string{}
	for i := range path.Points {
		id := fmt.Sprintf("shape:%d:%d", pathIndex, i)
		point, ok := projectedByID[id]
		if !ok {
			continue
		}
		x, y := point.X, point.Y
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("M %.6f %.6f", x, y))
		} else {
			parts = append(parts, fmt.Sprintf("L %.6f %.6f", x, y))
		}
	}
	return strings.Join(parts, " ")
}

func fitContentPadding(width, height float64) float64 {
	size := width
	if height > size {
		size = height
	}
	if size <= 0 {
		return 0.01
	}
	return size * 0.04
}

func uniqueStationNodes(stations []*graphstructure.Node) []*graphstructure.Node {
	out := []*graphstructure.Node{}
	seen := map[string]bool{}
	for _, station := range stations {
		if station == nil || seen[station.ID] {
			continue
		}
		seen[station.ID] = true
		out = append(out, station)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func collectCoordIDsFromRailEdges(g *graphstructure.Graph) []string {
	set := map[string]struct{}{}
	for _, e := range g.Edges {
		if e.Kind != "rail" {
			continue
		}
		if strings.HasPrefix(e.From, "coord:") {
			set[e.From] = struct{}{}
		}
		if strings.HasPrefix(e.To, "coord:") {
			set[e.To] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type stationInfo struct {
	ID   string
	Name string
	Lon  float64
	Lat  float64
}

func collectStations(g *graphstructure.Graph) []stationInfo {
	out := []stationInfo{}
	for _, n := range g.Nodes {
		if n.Kind != "station" {
			continue
		}
		out = append(out, stationInfo{
			ID:   n.ID,
			Name: n.Name,
			Lon:  n.Lon,
			Lat:  n.Lat,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

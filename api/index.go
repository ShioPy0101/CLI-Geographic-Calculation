package handler

import (
	"CLI-Geographic-Calculation/pkg/dataResolve"
	"CLI-Geographic-Calculation/pkg/giocal"
	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/linefilter"
	"CLI-Geographic-Calculation/pkg/giocal/railshape"
	"CLI-Geographic-Calculation/pkg/giocal/routepath"
	"CLI-Geographic-Calculation/pkg/giocal/sqlreq"
	"CLI-Geographic-Calculation/pkg/render/graphsvg"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type routeKey struct {
	year     int
	Resource string
}

type Dataset struct {
	Handler   datasetHandler
	Resources giocaltype.DatasetResourcePath
}

var datasets = map[routeKey]Dataset{
	{year: 2023, Resource: "rail"}: {
		Handler: handleRail,
		// DevResources: giocaltype.DatasetResourcePath{
		// 	Rail:    "pkg/giodata_public/N02-23_RailroadSection.json",
		// 	Station: "pkg/giodata_public/N02-23Station.json",
		// },
		// ProdResources: giocaltype.DatasetResourcePath{
		// 	Rail:    "https://github.com/Shio3001/giojson/blob/main/N02-23_RailroadSection.json",
		// 	Station: "https://github.com/Shio3001/giojson/blob/main/N02-23_Station.json",
		// },

		Resources: giocaltype.DatasetResourcePath{
			Rail:    "N02-23_RailroadSection.json",
			Station: "N02-23_Station.json",
		},
	},
}

const railroadShapePath = "pkg/giodata/N05-24_RailroadSection2.geojson"

func parseYearResourceFormat(path string) (year int, resource string, format string, err error) {
	p := strings.Trim(path, "/")
	parts := strings.Split(p, "/")

	if len(parts) != 2 && len(parts) != 3 {
		return 0, "", "", errBadPath
	}

	year, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", err
	}
	resource = parts[1]

	format = "json"
	if len(parts) == 3 && parts[2] != "" {
		format = strings.ToLower(parts[2])
	}
	return year, resource, format, nil
}

var errBadPath = errors.New("path must be {year}/{resource} or {year}/{resource}/{format}")

func Handler(w http.ResponseWriter, r *http.Request) {
	println("[HANDLER] urlPath:", r.URL.Path, "pathQuery:", r.URL.Query().Get("path"))

	p := r.URL.Query().Get("path")
	if p == "" {
		p = strings.TrimPrefix(r.URL.Path, "/api/")
	}

	year, resource, format, err := parseYearResourceFormat(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key := routeKey{
		Resource: resource,
		year:     year,
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "missing query parameter: query", http.StatusBadRequest)
		return
	}

	ds, ok := datasets[key]
	if !ok {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}

	resolved, err := resolveResources(ds.Resources)
	if err != nil {
		http.Error(w, "failed to resolve dataset resources: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ds.Handler(w, year, resolved, nil, query, format, boolQuery(r, "single_line") || boolQuery(r, "single-line"))
}

func boolQuery(r *http.Request, key string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolveResources(r giocaltype.DatasetResourcePath) (giocaltype.DatasetResourcePath, error) {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	if env == "" {
		env = "dev"
	}

	if env == "prod" {
		cacheDir := filepath.Join(os.TempDir(), "gio-cache")

		p := dataResolve.BlobURLProvider{
			CacheDir: cacheDir,
			Client:   &http.Client{Timeout: 20 * time.Second},
		}
		return p.Resolve(r)
	}

	// dev: ローカル baseDir を付けるだけ
	base := os.Getenv("GIO_LOCAL_BASE")
	if base == "" {
		base = "pkg/giodata_public"
	}
	return giocaltype.DatasetResourcePath{
		Rail:    filepath.Join(base, r.Rail),
		Station: filepath.Join(base, r.Station),
	}, nil
}

type datasetHandler func(w http.ResponseWriter, year int, res giocaltype.DatasetResourcePath, parsed *pg_query.ParseResult, rawSQL string, format string, singleLine bool)

func handleRail(
	w http.ResponseWriter,
	year int,
	res giocaltype.DatasetResourcePath,
	parsed *pg_query.ParseResult,
	rawSQL string,
	format string,
	singleLine bool,
) {
	// 1) データセット読み込み
	drs, err := giocal.LoadDatasetResource(res)
	if err != nil {
		http.Error(w, "Failed to load DatasetResource: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2) SQLLike/SQL -> Graph
	routeQuery, routeParseErr := sqlreq.ParseRouteQuery(rawSQL)
	var graph *graphstructure.Graph
	var renderPaths []routepath.RenderPath
	var geographicPaths []railshape.PathGeometry
	var routeStations []*graphstructure.Node
	if routeParseErr == nil {
		graph, err = sqlreq.RouteSelectionsToGraph(routeQuery.Selections, drs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resolvedRoute, err := sqlreq.RouteSelectionsToResolvedRoute(routeQuery.Selections, drs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		singleLine = singleLine || routeQuery.Options.SingleLine
		if routeQuery.Options.Geographic {
			shapeResolver, err := railshape.Load(railroadShapePath)
			if err != nil {
				http.Error(w, "failed to load railroad shape geojson: "+err.Error(), http.StatusInternalServerError)
				return
			}
			geographicPaths, err = shapeResolver.Resolve(resolvedRoute)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if singleLine {
				flattened, err := railshape.FlattenContinuous(geographicPaths)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				geographicPaths = []railshape.PathGeometry{flattened}
			}
			if routeQuery.Options.EndpointLabels {
				routeStations = endpointStationsFromResolvedRoute(resolvedRoute)
			} else {
				routeStations = stationsFromResolvedRoute(resolvedRoute)
			}
		} else if singleLine {
			path, err := routepath.FlattenContinuousRoute(resolvedRoute, routepath.FlattenOptions{AllowReverse: true})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			renderPaths = []routepath.RenderPath{path}
		} else {
			renderPaths = routepath.RenderPathsFromResolvedRoute(resolvedRoute)
		}
	} else {
		if parsed == nil {
			parsed, err = sqlreq.ParseSQLQueryE(rawSQL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		graph = sqlreq.SQLToGraph(
			linefilter.FilterRailroadSectionByProperties,
			parsed,
			drs,
		)
	}
	switch format {
	case "svg":
		options := graphsvg.Options{
			Width:        1200,
			Height:       800,
			Padding:      20,
			DrawStations: true,
			DrawLabels:   true,
		}
		if routeParseErr == nil && routeQuery.Options.NoLabels {
			options.DrawLabels = false
		}
		if routeParseErr == nil && routeQuery.Options.EndpointLabels {
			options.DrawStations = true
			options.DrawLabels = true
		}
		if routeParseErr == nil && routeQuery.Options.Geographic && !routeQuery.Options.NoAnimation {
			options.AnimatePath = true
		}
		var svg string
		if len(geographicPaths) > 0 {
			svg, err = graphsvg.RenderRailGeometriesSVG(geographicPaths, routeStations, options)
		} else if len(renderPaths) > 0 {
			svg, err = graphsvg.RenderRailPathsSVG(renderPaths, options)
		} else {
			svg, err = graphsvg.RenderRailGraphSVG(graph, options)
		}
		if err != nil {
			http.Error(w, "failed to render svg: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		_, _ = w.Write([]byte(svg))
		return

	case "json", "":
		// fallthrough
	default:
		http.Error(w, "unsupported format: "+format, http.StatusBadRequest)
		return
	}

	// JSON 返却
	out := map[string]any{
		"ok":       true,
		"year":     year,
		"resource": "rail",
		"sql":      rawSQL,
		"resolved_paths": map[string]string{
			"rail":    res.Rail,
			"station": res.Station,
		},
		"graph": graph,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

func stationsFromResolvedRoute(route routepath.ResolvedRoute) []*graphstructure.Node {
	out := []*graphstructure.Node{}
	for _, segment := range route.Segments {
		out = append(out, segment.Stations...)
	}
	return out
}

func endpointStationsFromResolvedRoute(route routepath.ResolvedRoute) []*graphstructure.Node {
	out := []*graphstructure.Node{}
	for _, segment := range route.Segments {
		if len(segment.Stations) == 0 {
			continue
		}
		out = appendStationEndpoint(out, segment.Stations[0])
		if len(segment.Stations) > 1 {
			out = appendStationEndpoint(out, segment.Stations[len(segment.Stations)-1])
		}
	}
	return out
}

func appendStationEndpoint(stations []*graphstructure.Node, station *graphstructure.Node) []*graphstructure.Node {
	if station == nil {
		return stations
	}
	for _, existing := range stations {
		if existing == nil {
			continue
		}
		if existing.ID == station.ID || (existing.Name == station.Name && existing.Lon == station.Lon && existing.Lat == station.Lat) {
			return stations
		}
	}
	return append(stations, station)
}

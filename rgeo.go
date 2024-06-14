/*
Copyright 2020 Sam Smith

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License.  You may obtain a copy of the
License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied.  See the License for the
specific language governing permissions and limitations under the License.
*/

/*
Package rgeo is a fast, simple solution for local reverse geocoding.

Rather than relying on external software or online APIs, rgeo packages all of
the data it needs in your binary. This means it will only works down to the
level of cities, but if that's all you need then this is the library for you.

rgeo uses data from https://naturalearthdata.com, if your coordinates are going
to be near specific borders I would advise checking the data beforehand (links
to which are in the files). If you want to use your own dataset, check out the
datagen folder.

# Installation

	go get github.com/sams96/rgeo

# Contributing

Contributions are welcome, I haven't got any guidelines or anything so maybe
just make an issue first.
*/
package rgeo

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
	"github.com/twpayne/go-geom"
)

// ErrLocationNotFound is returned when no country is found for given
// coordinates.
var ErrLocationNotFound = errors.New("country not found")

// Location is the return type for ReverseGeocode.
type Location struct {
	// Commonly used country name
	Country string `json:"country,omitempty"`

	// Formal name of country
	CountryLong string `json:"country_long,omitempty"`

	// ISO 3166-1 alpha-1 and alpha-2 codes
	CountryCode2 string `json:"country_code_2,omitempty"`
	CountryCode3 string `json:"country_code_3,omitempty"`

	Continent string `json:"continent,omitempty"`
	Region    string `json:"region,omitempty"`
	SubRegion string `json:"subregion,omitempty"`

	Province string `json:"province,omitempty"`

	// ISO 3166-2 code
	ProvinceCode string `json:"province_code,omitempty"`

	City string `json:"city,omitempty"`
}

// Rgeo is the type used to hold pre-created polygons for reverse geocoding.
type Rgeo struct {
	index         *s2.ShapeIndex
	makeEdgeQuery func() *s2.EdgeQuery
}

// shapeLocation is used for storing location references in s2.ShapeIndex.
type shapeLocation interface {
	s2.Shape
	Location() Location
}

// shape implements shapeLocation
type shape struct {
	s2.Shape
	loc Location
}

func (s *shape) Location() Location {
	return s.loc
}

// Dataset provides a Feature slice.
// It is a function for easier integration into existing rgeo v1 code only.
type Dataset func() []Feature

// New returns a Rgeo struct which can then be used with ReverseGeocode.
// It takes any number of datasets as arguments.
//
// The included datasets are:
//   - Cities10
//   - Countries10
//   - Countries110
//   - Provinces10
func New(datasets ...Dataset) (*Rgeo, error) {
	if len(datasets) == 0 {
		return nil, errors.New("no datasets provided")
	}
	r := &Rgeo{index: s2.NewShapeIndex()}
	r.SetSnappingDistanceEarth(5) // kilometers on Earth
	for _, dataset := range datasets {
		features := dataset()
		for _, f := range features {
			p := f.Polygon
			r.index.Add(&shape{Shape: p, loc: f.Location})
		}
	}
	return r, nil
}

// Build builds the underlying shape index. This ensures that future calls to
// ReverseGeocode will be fast. If Build is not called, then the first lookup
// will build the index implicitly and experience a 1s+ delay.
func (r *Rgeo) Build() {
	r.index.Build()
}

// SetSnappingDistanceEarth sets ReverseGeocodeSnapping snap distance on Earth.
// Only edges within the defined radius around given points will be considered
// by ReverseGeocodeSnapping.
//
// The input is the snapping distance in kilometers. Must be positive.
func (r *Rgeo) SetSnappingDistanceEarth(d float64) {
	const earthRadiusKM = 6371
	r.SetSnappingDistanceCustom(d, earthRadiusKM)
}

// SetSnappingDistanceCustom recalculates the underlying ChordAngle for the
// DistanceLimit of nearest-edge queries via ReverseGeocodeSnapping.
//
// The inputs are the snapping distance on the sphere's surface in kilometers,
// and the radius of the sphere used in the dataset.
func (r *Rgeo) SetSnappingDistanceCustom(d float64, radius float64) {
	angle := math.Sin(d / radius)
	options := s2.NewClosestEdgeQueryOptions().
		MaxResults(1).
		DistanceLimit(s1.ChordAngleFromAngle(s1.Angle(angle)).Successor())
	r.makeEdgeQuery = func() *s2.EdgeQuery {
		return s2.NewClosestEdgeQuery(r.index, options)
	}
}

// ReverseGeocode returns the country in which the given coordinate is located.
//
// The input is a geom.Coord, which is just a []float64 with the longitude
// in the zeroth position and the latitude in the first position
// (i.e. []float64{lon, lat}).
func (r *Rgeo) ReverseGeocode(loc geom.Coord) (Location, error) {
	query := s2.NewContainsPointQuery(r.index, s2.VertexModelOpen)
	res := query.ContainingShapes(pointFromCoord(loc))
	if len(res) == 0 {
		return Location{}, ErrLocationNotFound
	}

	return r.combineLocations(res), nil
}

func (r *Rgeo) ReverseGeocodeSnapping(coord geom.Coord) (Location, error) {
	// Try to get a hit first, i.e. we are already in a country
	loc, err := r.ReverseGeocode(coord)
	if err == nil {
		return loc, nil
	} else if !errors.Is(err, ErrLocationNotFound) {
		return Location{}, err
	}

	// Not in a country, so look for the closest country in the defined margin
	point := pointFromCoord(coord)
	res := r.makeEdgeQuery().FindEdges(s2.NewMinDistanceToPointTarget(point))
	if len(res) == 0 {
		return Location{}, ErrLocationNotFound
	}

	// Get shape of the closest country in our margin
	shape := r.index.Shape(res[0].ShapeID())
	if shape == nil {
		return Location{}, ErrLocationNotFound
	}

	return r.combineLocations([]s2.Shape{shape}), nil
}

// combineLocations combines the Locations for the given s2 Shapes.
func (r *Rgeo) combineLocations(shapes []s2.Shape) (l Location) {
	for _, s := range shapes {
		loc := s.(shapeLocation).Location()
		l = Location{
			Country:      firstNonEmpty(l.Country, loc.Country),
			CountryLong:  firstNonEmpty(l.CountryLong, loc.CountryLong),
			CountryCode2: firstNonEmpty(l.CountryCode2, loc.CountryCode2),
			CountryCode3: firstNonEmpty(l.CountryCode3, loc.CountryCode3),
			Continent:    firstNonEmpty(l.Continent, loc.Continent),
			Region:       firstNonEmpty(l.Region, loc.Region),
			SubRegion:    firstNonEmpty(l.SubRegion, loc.SubRegion),
			Province:     firstNonEmpty(l.Province, loc.Province),
			ProvinceCode: firstNonEmpty(l.ProvinceCode, loc.ProvinceCode),
			City:         firstNonEmpty(l.City, loc.City),
		}
	}

	return
}

// firstNonEmpty returns the first non empty parameter.
func firstNonEmpty(s ...string) string {
	for _, i := range s {
		if i != "" {
			return i
		}
	}

	return ""
}

// Get the relevant strings from the GeoJSON properties.
func getLocationStrings(p map[string]interface{}) Location {
	return Location{
		Country:      getPropertyString(p, "ADMIN", "admin"),
		CountryLong:  getPropertyString(p, "FORMAL_EN"),
		CountryCode2: getPropertyString(p, "ISO_A2_EH"),
		CountryCode3: getPropertyString(p, "ISO_A3_EH"),
		Continent:    getPropertyString(p, "CONTINENT"),
		Region:       getPropertyString(p, "REGION_UN"),
		SubRegion:    getPropertyString(p, "SUBREGION"),
		Province:     getPropertyString(p, "name"),
		ProvinceCode: getPropertyString(p, "iso_3166_2"),
		City:         strings.TrimSuffix(getPropertyString(p, "name_conve"), "2"),
	}
}

// getPropertyString gets the value from a map given the key as a string, or
// from the next given key if the previous fails.
func getPropertyString(m map[string]interface{}, keys ...string) (s string) {
	var ok bool
	for _, k := range keys {
		s, ok = m[k].(string)
		if ok {
			break
		}
	}

	return
}

// polygonFromGeometry converts a geom.T to an s2 Polygon.
func polygonFromGeometry(g geom.T) (*s2.Polygon, error) {
	var (
		polygon *s2.Polygon
		err     error
	)

	switch t := g.(type) {
	case *geom.Polygon:
		polygon, err = polygonFromPolygon(t)
	case *geom.MultiPolygon:
		polygon, err = polygonFromMultiPolygon(t)
	default:
		return nil, errors.New("needs Polygon or MultiPolygon")
	}

	if err != nil {
		return nil, err
	}

	return polygon, nil
}

// Converts a geom MultiPolygon to an s2 Polygon.
func polygonFromMultiPolygon(p *geom.MultiPolygon) (*s2.Polygon, error) {
	loops := make([]*s2.Loop, 0, p.NumPolygons())

	for i := 0; i < p.NumPolygons(); i++ {
		this, err := loopSliceFromPolygon(p.Polygon(i))
		if err != nil {
			return nil, err
		}

		loops = append(loops, this...)
	}

	return s2.PolygonFromLoops(loops), nil
}

// Converts a geom Polygon to an s2 Polygon.
func polygonFromPolygon(p *geom.Polygon) (*s2.Polygon, error) {
	loops, err := loopSliceFromPolygon(p)
	return s2.PolygonFromLoops(loops), err
}

// Converts a geom Polygon to slice of s2 Loop.
//
// Modified from types.loopFromPolygon from github.com/dgraph-io/dgraph.
func loopSliceFromPolygon(p *geom.Polygon) ([]*s2.Loop, error) {
	loops := make([]*s2.Loop, 0, p.NumLinearRings())

	for i := 0; i < p.NumLinearRings(); i++ {
		r := p.LinearRing(i)
		n := r.NumCoords()

		if n < 4 {
			return nil, errors.New("can't convert ring with less than 4 points")
		}

		if !r.Coord(0).Equal(geom.XY, r.Coord(n-1)) {
			return nil, fmt.Errorf(
				"last coordinate not same as first for polygon: %+v", p.FlatCoords())
		}

		// S2 specifies that the orientation of the polygons should be CCW.
		// However there is no restriction on the orientation in WKB (or
		// GeoJSON). To get the correct orientation we assume that the polygons
		// are always less than one hemisphere. If they are bigger, we flip the
		// orientation.
		reverse := isClockwise(r)
		l := loopFromRing(r, reverse)

		// Since our clockwise check was approximate, we check the cap and
		// reverse if needed.
		if l.CapBound().Radius().Degrees() > 90 {
			// Remaking the loop sometimes caused problems, this works better
			l.Invert()
		}

		loops = append(loops, l)
	}

	return loops, nil
}

// Checks if a ring is clockwise or counter-clockwise. Note: This uses the
// algorithm for planar polygons and doesn't work for spherical polygons that
// contain the poles or the antimeridan discontinuity. We use this as a fast
// approximation instead.
//
// From github.com/dgraph-io/dgraph
func isClockwise(r *geom.LinearRing) bool {
	// The algorithm is described here
	// https://en.wikipedia.org/wiki/Shoelace_formula
	var a float64

	n := r.NumCoords()

	for i := 0; i < n; i++ {
		p1 := r.Coord(i)
		p2 := r.Coord((i + 1) % n)
		a += (p2.X() - p1.X()) * (p1.Y() + p2.Y())
	}

	return a > 0
}

// From github.com/dgraph-io/dgraph
func loopFromRing(r *geom.LinearRing, reverse bool) *s2.Loop {
	// In WKB, the last coordinate is repeated for a ring to form a closed loop.
	// For s2 the points aren't allowed to repeat and the loop is assumed to be
	// closed, so we skip the last point.
	n := r.NumCoords()
	pts := make([]s2.Point, n-1)

	for i := 0; i < n-1; i++ {
		var c geom.Coord
		if reverse {
			c = r.Coord(n - 1 - i)
		} else {
			c = r.Coord(i)
		}

		pts[i] = pointFromCoord(c)
	}

	return s2.LoopFromPoints(pts)
}

// From github.com/dgraph-io/dgraph
func pointFromCoord(r geom.Coord) s2.Point {
	// The GeoJSON spec says that coordinates are specified as [long, lat]
	// We assume that any data encoded in the database follows that format.
	ll := s2.LatLngFromDegrees(r.Y(), r.X())
	return s2.PointFromLatLng(ll)
}

// String method for type Location.
func (l Location) String() string {
	ret := "<Location>"

	// Special case for empty location
	if l == (Location{}) {
		return ret + " Empty Location"
	}

	// Add city name
	if l.City != "" {
		ret += " " + l.City + ","
	}

	// Add province name
	if l.Province != "" {
		ret += " " + l.Province + ","
	}

	// Add country name
	if l.Country != "" {
		ret += " " + l.Country
	} else if l.CountryLong != "" {
		ret += " " + l.CountryLong
	}

	// Add country code in brackets
	if l.CountryCode3 != "" {
		ret += " (" + l.CountryCode3 + ")"
	} else if l.CountryCode2 != "" {
		ret += " (" + l.CountryCode2 + ")"
	}

	// Add continent/region
	if len(ret) > len("<Location>") {
		ret += ","
	}

	switch {
	case l.Continent != "":
		ret += " " + l.Continent
	case l.Region != "":
		ret += " " + l.Region
	case l.SubRegion != "":
		ret += " " + l.SubRegion
	}

	return ret
}

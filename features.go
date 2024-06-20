package rgeo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/golang/geo/s2"
	"github.com/twpayne/go-geom/encoding/geojson"
)

type FeatureCollection []Feature

func (fc *FeatureCollection) Encode(w io.Writer) error {
	for _, f := range *fc {
		if err := f.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

type Feature struct {
	Location Location
	Polygon  *s2.Polygon
}

func (f *Feature) Encode(w io.Writer) error {
	// Neither JSON nor s2.Polygon have self-terminating encoding implementations, meh.
	// Format is thus <len><location json> <len><polygon> for each feature.

	if locBuf, err := json.Marshal(f.Location); err != nil {
		return fmt.Errorf("encode location: %w", err)
	} else if err := binary.Write(w, binary.LittleEndian, uint32(len(locBuf))); err != nil {
		return fmt.Errorf("write size: %w", err)
	} else if _, err := w.Write(locBuf); err != nil {
		return fmt.Errorf("write location: %w", err)
	}

	polyBuf := bytes.NewBuffer(nil)
	if err := f.Polygon.Encode(polyBuf); err != nil {
		return fmt.Errorf("encode polygon: %w", err)
	} else if err := binary.Write(w, binary.LittleEndian, uint32(polyBuf.Len())); err != nil {
		return fmt.Errorf("write size: %w", err)
	} else if _, err := polyBuf.WriteTo(w); err != nil {
		return fmt.Errorf("write polygon: %w", err)
	}

	return nil
}

func (f *Feature) Decode(r io.Reader) error {
	var l uint32

	if err := binary.Read(r, binary.LittleEndian, &l); err != nil {
		return fmt.Errorf("read location length: %w", err)
	}
	locBuf := make([]byte, l)
	if _, err := io.ReadFull(r, locBuf); err != nil {
		return fmt.Errorf("read location: %w", unexpectedEOF(err))
	}
	if err := json.Unmarshal(locBuf, &f.Location); err != nil {
		return fmt.Errorf("decode location: %w", unexpectedEOF(err))
	}

	if err := binary.Read(r, binary.LittleEndian, &l); err != nil {
		return fmt.Errorf("read polygon length: %w", unexpectedEOF(err))
	}
	polyBuf := make([]byte, l)
	if _, err := io.ReadFull(r, polyBuf); err != nil {
		return fmt.Errorf("read polygon: %w", unexpectedEOF(err))
	}
	f.Polygon = &s2.Polygon{}
	if err := f.Polygon.Decode(bytes.NewReader(polyBuf)); err != nil {
		return fmt.Errorf("bad polygon in geometry: %w", unexpectedEOF(err))
	}

	return nil
}

func LoadEncoded(r io.Reader) ([]Feature, error) {
	var result []Feature
	for i := 0; ; i++ {
		var f Feature
		if err := f.Decode(r); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode feature %d: %w", i, err)
		}
		result = append(result, f)
	}
	return result, nil
}

func LoadGeoJSON(fc geojson.FeatureCollection) (FeatureCollection, error) {
	features := make(FeatureCollection, 0, len(fc.Features))
	for _, f := range fc.Features {
		poly, err := polygonFromGeometry(f.Geometry)
		if err != nil {
			return nil, fmt.Errorf("bad polygon in geometry: %w", err)
		}
		features = append(features, Feature{
			Location: getLocationStrings(f.Properties),
			Polygon:  poly,
		})
	}
	return features, nil
}

func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	} else {
		return err
	}
}

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
Command datagen converts GeoJSON files into go files containing functions that
return the compressed GeoJSON, it can also merge properties from one GeoJSON
file into another using the -merge flag (which it does by matching the country
names). You can use this if you want to use a different dataset to any of those
included, although that might be somewhat awkward if the properties in your
GeoJSON file are different.
*/
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/twpayne/go-geom/encoding/geojson"
)

func main() {
	outPath := flag.String("o", "", "path to output file")
	propsFilePath := flag.String("merge", "", "path to file to merge properties from")
	flag.Parse()

	if *outPath == "" {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s -o outprefix <infile.geojson> [infile2.geojson] [...]\n", os.Args[0])
		os.Exit(1)
	}

	inputFiles := flag.Args()

	// cosmetics for generating the .txt attribution file
	attributionFiles := append([]string{}, inputFiles...)
	if *propsFilePath != "" {
		attributionFiles = append(attributionFiles, *propsFilePath)
	}
	for i, path := range attributionFiles {
		attributionFiles[i] = filepath.Base(path)
	}

	if fc, err := readInputs(inputFiles, *propsFilePath); err != nil {
		log.Fatal("error reading inputs: ", err)
	} else if err := writeFeatures(*outPath, fc); err != nil {
		log.Fatal("error writing features: ", err)
	} else if err := writeAttribution(*outPath, attributionFiles); err != nil {
		log.Fatal("error writing attribution: ", err)
	}
}

func readInputs(in []string, propsFileName string) (*geojson.FeatureCollection, error) {
	var props *geojson.FeatureCollection
	if propsFileName != "" {
		md, err := readGeoJSON(propsFileName)
		if err != nil {
			return nil, fmt.Errorf("read props GeoJSON file: %w", err)
		}
		props = md
	}

	fc := &geojson.FeatureCollection{}
	for _, f := range in {
		s, err := readGeoJSON(f)
		if err != nil {
			return nil, fmt.Errorf("read input GeoJSON file: %w", err)
		}
		if props != nil {
			if err := extendProps(s, props); err != nil {
				return nil, fmt.Errorf("extend properties: %w", err)
			}
		}
		fc.Features = append(fc.Features, s.Features...)
	}

	return fc, nil
}

func writeFeatures(outPath string, fc *geojson.FeatureCollection) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() { _ = f.Close() }()

	zw, err := gzip.NewWriterLevel(f, 9)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	defer func() { _ = zw.Close() }()
	encoded, err := json.Marshal(fc) // because NewEncoder would append a \n
	if err != nil {
		return fmt.Errorf("encode GeoJSON: %w", err)
	}
	if _, err := zw.Write(encoded); err != nil {
		return fmt.Errorf("write GeoJSON: %w", err)
	}

	// explicit flush so that zw.Close always succeeds
	if err := zw.Flush(); err != nil {
		return fmt.Errorf("flush gzip writer: %w", err)
	}

	return nil
}

// readGeoJSON parses a GeoJSON file as geojson.FeatureCollection
func readGeoJSON(path string) (*geojson.FeatureCollection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var result geojson.FeatureCollection
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode GeoJSON: %w", err)
	}

	return &result, nil
}

// extendProps merges properties from source into dest based on country name
func extendProps(dest *geojson.FeatureCollection, source *geojson.FeatureCollection) error {
	for _, feat := range dest.Features {
		destCountry, ok := getAdmin(feat)
		if !ok {
			return fmt.Errorf("missing country in destination feature: %v", feat)
		}
		for _, md := range source.Features {
			if sourceCountry, _ := getAdmin(md); sourceCountry == destCountry {
				for k, v := range md.Properties {
					feat.Properties[k] = v
				}
			}
		}
	}
	return nil
}

// getAdmin compensates for "admin" vs "ADMIN"
func getAdmin(feat *geojson.Feature) (string, bool) {
	if country, ok := feat.Properties["ADMIN"].(string); ok {
		return country, true
	} else if country, ok := feat.Properties["admin"].(string); ok {
		return country, true
	} else {
		return "", false
	}
}

func writeAttribution(outPath string, attribFiles []string) error {
	basePath := strings.TrimSuffix(outPath, filepath.Ext(outPath))
	f, err := os.Create(basePath + ".txt")
	if err != nil {
		return fmt.Errorf("create attribution file: %w", err)
	}
	defer func() { _ = f.Close() }()
	fileName := filepath.Base(basePath)
	if _, err := fmt.Fprintf(f, "%s uses data from %s", fileName, printSlice(attribFiles)); err != nil {
		return fmt.Errorf("write attribution file: %w", err)
	}
	return nil
}

// printSlice prints a slice of strings with commas and an ampersand if needed
func printSlice(in []string) string {
	n := len(in)
	switch n {
	case 0:
		return ""
	case 1:
		return in[0]
	case 2:
		return strings.Join(in, " & ")
	default:
		return printSlice([]string{strings.Join(in[:n-1], ", "), in[n-1]})
	}
}

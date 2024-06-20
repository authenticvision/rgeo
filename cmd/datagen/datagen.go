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

Usage

	go run datagen.go -o outfile.go infile.geojson

The variable containing the data will be named outfile.

rgeo reads the location information from the following GeoJSON properties:

	- Country:      "ADMIN" or "admin"
	- CountryLong:  "FORMAL_EN"
	- CountryCode2: "ISO_A2"
	- CountryCode3: "ISO_A3"
	- Continent:    "CONTINENT"
	- Region:       "REGION_UN"
	- SubRegion:    "SUBREGION"
	- Province:     "name"
	- ProvinceCode: "iso_3166_2"
	- City:         "name_conve"
*/
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/twpayne/go-geom/encoding/geojson"
)

func main() {
	// Read args
	outFileName := flag.String("o", "", "Path to output file")
	neCommentFlag := flag.Bool("ne", false, "Use Natural earth comment")
	mergeFileName := flag.String("merge", "", "File to get extra info from")

	flag.Parse()

	if *outFileName == "" {
		fmt.Println("Please specify an output file with -o")
		return
	}

	feats, err := readInputs(flag.Args(), *mergeFileName)
	if err != nil {
		log.Fatal(err)
	}

	var pre string
	if *neCommentFlag {
		pre = "https://github.com/nvkelso/natural-earth-vector/blob/master/geojson/"
	}

	files := flag.Args()
	if *mergeFileName != "" {
		files = append(files, *mergeFileName)
	}

	resp, err := json.Marshal(feats)
	if err != nil {
		log.Fatal(err)
	}

	// Compress data
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, 9)

	if _, err := zw.Write(resp); err != nil {
		log.Fatal(err)
	}
	zw.Flush()
	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}

	f, _ := os.Create(fmt.Sprintf("%s.gz", *outFileName))
	_, err = io.Copy(f, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fReadme, _ := os.Create(fmt.Sprintf("%s.txt", *outFileName))
	fmt.Fprintf(fReadme, "%s %s", strings.TrimSuffix(*outFileName, ".go"), "uses data from "+printSlice(prefixSlice(pre, files)))
}

func readInputs(in []string, mergeFileName string) (*geojson.FeatureCollection, error) {
	fc := new(geojson.FeatureCollection)

	var mergeData *geojson.FeatureCollection

	if mergeFileName != "" {
		md, err := readInput(mergeFileName, nil)
		if err != nil {
			return nil, err
		}

		mergeData = md
	}

	for _, f := range in {
		s, err := readInput(f, mergeData)
		if err != nil {
			return nil, err
		}

		fc.Features = append(fc.Features, s.Features...)
	}

	return fc, nil
}

func readInput(f string, mergeData *geojson.FeatureCollection) (*geojson.FeatureCollection, error) {
	// Open infile
	infile, err := os.Open(f)
	if err != nil {
		return nil, err
	}

	defer infile.Close()

	// Parse geojson
	var fc geojson.FeatureCollection
	if err := json.NewDecoder(infile).Decode(&fc); err != nil {
		return nil, err
	}

	if mergeData == nil {
		return &fc, nil
	}

	for _, feat := range fc.Features {
		country, ok := feat.Properties["admin"].(string)
		if !ok {
			log.Println("country name in wrong place")
			break
		}

		for _, md := range mergeData.Features {
			if md.Properties["ADMIN"] == country {
				for k, v := range md.Properties {
					feat.Properties[k] = v
				}
			}
		}
	}

	return &fc, nil
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

// prefix slice adds a given prefix to a slice of strings
func prefixSlice(pre string, slice []string) (ret []string) {
	for _, i := range slice {
		ret = append(ret, pre+i)
	}

	return
}

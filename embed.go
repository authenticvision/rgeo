//go:build !datagen

package rgeo

// data is sourced from: https://github.com/nvkelso/natural-earth-vector/tree/master/geojson
// and bundled via regen.sh in the repository's root

// embedding files individually here to allow the linker to strip out unused ones
import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

//go:embed data/Cities10.zst
var cities10 []byte

func Cities10() []Feature {
	return must(embeddedFeatureCollection(cities10))
}

//go:embed data/Countries10.zst
var countries10 []byte

func Countries10() []Feature {
	return must(embeddedFeatureCollection(countries10))
}

//go:embed data/Countries110.zst
var countries110 []byte

func Countries110() []Feature {
	return must(embeddedFeatureCollection(countries110))
}

//go:embed data/Provinces10.zst
var provinces10 []byte

func Provinces10() []Feature {
	return must(embeddedFeatureCollection(provinces10))
}

func must(features []Feature, err error) []Feature {
	if err != nil {
		panic("rgeo embed.go: " + err.Error())
	}
	return features
}

func embeddedFeatureCollection(data []byte) ([]Feature, error) {
	br := bytes.NewReader(data)
	if br.Len() == 0 {
		return nil, errors.New("empty dataset")
	}

	zr, err := zstd.NewReader(br)
	if err != nil {
		return nil, fmt.Errorf("zstd reader setup: %w", err)
	}
	defer zr.Close()

	result, err := LoadEncoded(zr)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return result, nil
}

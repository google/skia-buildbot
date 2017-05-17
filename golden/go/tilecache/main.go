package main

import (
	"os"

	"github.com/bradfitz/gomemcache/memcache"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/serialize"
)

func main() {

	mc := memcache.New("10.0.0.1:11211", "10.0.0.2:11211", "10.0.0.3:11212")
	mc.Set(&memcache.Item{Key: "foo", Value: []byte("my value")})

	it, err := mc.Get("gold-tile")
}

func getTile(path string) (*tiling.Tile, error) {
	loadTimer := timer.New("Loading sample tile")
	sampledState, err := loadSample(path)
	if err != nil {
		return nil, err
	}
	loadTimer.Stop()
	return sampledState.Tile, nil
}

func loadSample(fileName string) (*serialize.Sample, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	sample, err := serialize.DeserializeSample(file)
	if err != nil {
		return nil, err
	}
	return sample, nil
}

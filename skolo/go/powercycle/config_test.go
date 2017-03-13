package main

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config, err := DevicesFromTomlFile("./example.toml")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	for id, conf := range config.MPower {
		fmt.Printf("MPower: %s :  %s\n", id, spew.Sprint(conf))
	}
	for id, conf := range config.EdgeSwitch {
		fmt.Printf("Edge: %s :  %s\n", id, spew.Sprint(conf))
	}
}

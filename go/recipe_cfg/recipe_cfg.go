package recipe_cfg

import (
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/go/util"
)

/*
	Utilities for parsing the recipes.cfg file.
*/

const (
	RECIPE_CFG_PATH       = "infra/config/recipes.cfg"
	SUPPORTED_API_VERSION = 2
)

// RecipeDep represents a single recipe dependency.
type RecipeDep struct {
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
	Url      string `json:"url"`
}

// RecipesCfg represents the structure of the recipes.cfg file.
type RecipesCfg struct {
	ApiVersion  int                   `json:"api_version"`
	Deps        map[string]*RecipeDep `json:"deps"`
	ProjectId   string                `json:"project_id"`
	RecipesPath string                `json:"recipes_path"`
}

// Parse the given recipes.cfg file and return it.
func ParseCfg(cfg string) (*RecipesCfg, error) {
	var rv RecipesCfg
	if err := util.WithReadFile(cfg, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&rv)
	}); err != nil {
		return nil, err
	}
	if rv.ApiVersion != SUPPORTED_API_VERSION {
		return nil, fmt.Errorf("Got API version %d but only support %d!", rv.ApiVersion, SUPPORTED_API_VERSION)
	}
	return &rv, nil
}

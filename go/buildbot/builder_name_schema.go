package buildbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/gitiles"
)

// This file must be kept in sync with:
// https://chromium.googlesource.com/chromium/tools/build/+/master/scripts/common/skia/builder_name_schema.json

var (
	schema *builderNameSchema
)

type builderNameSchema struct {
	Schema           map[string][]string `json:"builder_name_schema"`
	BuilderNameSep   string              `json:"builder_name_sep"`
	TrybotNameSuffix string              `json:"trybot_name_suffix"`
}

func init() {
	buf := bytes.NewBuffer(nil)
	r := gitiles.NewRepo("https://skia.googlesource.com/skia")
	if err := r.ReadFile("infra/bots/recipe_modules/builder_name_schema/builder_name_schema.json", buf); err != nil {
		glog.Fatal(err)
	}
	res := new(builderNameSchema)
	if err := json.NewDecoder(buf).Decode(res); err != nil {
		glog.Fatal(err)
	}
	schema = res
}

func ParseBuilderName(name string) (map[string]string, error) {
	split := strings.Split(name, schema.BuilderNameSep)
	if len(split) < 2 {
		return nil, fmt.Errorf("Invalid builder name: %q", name)
	}
	role := split[0]
	split = split[1:]
	keys, ok := schema.Schema[role]
	if !ok {
		return nil, fmt.Errorf("Invalid builder name; %q is not a valid role.", role)
	}
	isTrybot := false
	if split[len(split)-1] == schema.TrybotNameSuffix {
		isTrybot = true
		split = split[:len(split)-1]
	}
	extraConfig := ""
	if len(split) == len(keys)+1 {
		extraConfig = split[len(split)-1]
		split = split[:len(split)-1]
	}
	if len(split) != len(keys) {
		return nil, fmt.Errorf("Invalid builder name: %q has incorrect number of parts.", name)
	}
	rv := make(map[string]string, len(keys)+2)
	rv["role"] = role
	rv["is_trybot"] = strconv.FormatBool(isTrybot)
	if extraConfig != "" {
		rv["extra_config"] = extraConfig
	}
	for i, k := range keys {
		rv[k] = split[i]
	}
	return rv, nil
}

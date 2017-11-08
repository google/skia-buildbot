package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	source = flag.String("source", "", "The root JSON Schema file.")
)

type Items struct {
	Plurality string        // allOf, anyOf, ...
	Schemas   []*JSONSchema `json:"schemas"`
	Error     string        `json:"error"`
}

func (it Items) Refs() map[string]bool {
	ret := map[string]bool{}
	for _, s := range it.Schemas {
		r := s.Refs()
		for k, v := range r {
			ret[k] = v
		}
	}
	return ret
}

type OneOf struct {
	OneOf []*JSONSchema `json:"oneOf"`
}

func (i *Items) UnmarshalJSON(b []byte) error {
	arr := []*JSONSchema{}
	if err := json.Unmarshal(b, &arr); err != nil {
		oneOf := OneOf{}
		if err := json.Unmarshal(b, &oneOf); err != nil || len(oneOf.OneOf) == 0 {
			obj := &JSONSchema{}
			if err := json.Unmarshal(b, &obj); err != nil {
				sklog.Errorf("Unknown items: %s %s", err, b)
				i.Error = err.Error()
			} else {
				i.Schemas = []*JSONSchema{obj}
				i.Plurality = "exactly"
			}
		} else {
			i.Schemas = oneOf.OneOf
			i.Plurality = "oneOf"
		}
	} else {
		i.Schemas = arr
		i.Plurality = "exactly"
	}

	return nil
}

type Value struct {
	Int64 int64  `json:"int64"`
	Str   string `json:"str"`
}

func (v *Value) UnmarshalJSON(b []byte) error {
	sklog.Infof("Value::Unmarshal : %s", string(b))
	if i64, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		v.Int64 = i64
	} else {
		v.Str = string(b)
	}
	sklog.Info("Value found: %v", *v)
	return nil
}

type Property struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Items       Items  `json:"items"`
}

type JSONSchema struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Ref         string           `json:"$ref"`
	Schema      string           `json:"$schema"`
	Type        string           `json:"type"`
	Value       Value            `json:"value"`
	Items       Items            `json:"items"`
	StandsFor   string           `json:"standsFor"`
	Properties  map[string]Items `json:"properties"`
}

func (j JSONSchema) Refs() map[string]bool {
	if j.Ref != "" {
		return map[string]bool{j.Ref: true}
	} else {
		ret := map[string]bool{}
		for _, p := range j.Properties {
			r := p.Refs()
			for k, v := range r {
				ret[k] = v
			}
		}
		for _, i := range j.Items.Schemas {
			r := i.Refs()
			for k, v := range r {
				ret[k] = v
			}
		}
		return ret
	}
}

func loadSchema(filename string) (*Items, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s: %s", filename, err)
	}
	defer util.Close(f)

	/*
		schema := &JSONSchema{
			Properties: map[string]Property{},
		}
	*/

	schema := &Items{
		Schemas: []*JSONSchema{},
	}

	if err := json.NewDecoder(f).Decode(schema); err != nil {
		return nil, fmt.Errorf("Failed to decode: %s", err)
	}
	return schema, err
}

func main() {
	common.Init()

	if *source == "" {
		sklog.Fatal("The --source flag is required.")
	}

	allSchemas := map[string]*Items{}

	root := filepath.Dir(*source)

	schema, err := loadSchema(*source)
	if err != nil {
		sklog.Fatalf("Failed to open: %s", err)
	}
	allSchemas["#"] = schema

	// Resolve refs.
	s, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		sklog.Fatalf("Failed to pretty print: %s", err)
	}
	sklog.Infof("%s\n", s)
	allRefs := schema.Refs()
	newRefs := schema.Refs()
	newnewRefs := map[string]bool{}
	for len(newRefs) != 0 {
		sklog.Infof("newRefs %v\n", newRefs)
		for r, _ := range newRefs {
			filename := r[2:] + ".json"
			schema, err = loadSchema(filepath.Join(root, filename))
			if err != nil {
				sklog.Fatalf("Failed to load subschema %s: %s", filename, err)
			}
			allRefs[r] = true
			allSchemas[r] = schema
			subRefs := schema.Refs()
			for subref, _ := range subRefs {
				if !allRefs[subref] {
					sklog.Infof("Found %s in %s\n", subref, filename)
					newnewRefs[subref] = true
				}
			}
		}
		newRefs = newnewRefs
		for k, v := range newRefs {
			allRefs[k] = v
		}
		newnewRefs = map[string]bool{}
	}
	// Now emit some docs with cross-references.
	keys := []string{}
	for k, _ := range allSchemas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("%v", keys)
	for _, k := range keys {
		s, err := json.MarshalIndent(allSchemas[k], "", "  ")
		if err != nil {
			sklog.Fatalf("Failed to pretty print: %s", err)
		}
		fmt.Printf("---------------------------\n%s\n%s\n", k, s)
	}
}

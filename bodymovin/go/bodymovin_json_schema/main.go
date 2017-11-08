package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
				sklog.Fatalf("Unknown items: %s", b)
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

type Property struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Items       Items  `json:"items"`
}

type JSONSchema struct {
	Ref        string              `json:"$ref"`
	Schema     string              `json:"$schema"`
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
}

func (j JSONSchema) Refs() map[string]bool {
	if j.Ref != "" {
		return map[string]bool{j.Ref: true}
	} else {
		ret := map[string]bool{}
		for _, p := range j.Properties {
			for _, s := range p.Items.Schemas {
				r := s.Refs()
				for k, v := range r {
					ret[k] = v
				}
			}
		}
		return ret
	}
}

func loadSchema(filename string) (*JSONSchema, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s: %s", filename, err)
	}
	defer util.Close(f)

	schema := &JSONSchema{
		Properties: map[string]Property{},
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

	allSchemas := map[string]*JSONSchema{}

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
	fmt.Printf("%s\n", s)
	allRefs := schema.Refs()
	newRefs := schema.Refs()
	newnewRefs := map[string]bool{}
	for len(newRefs) != 0 {
		fmt.Printf("newRefs %v\n", newRefs)
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
					fmt.Printf("Found %s in %s\n", subref, filename)
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
	// Now emit some markdown docs with cross-references.
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Ref         string            `json:"$ref"`
	Schema      string            `json:"$schema"`
	Type        string            `json:"type"`
	Value       Value             `json:"value"`
	Items       Items             `json:"items"`
	StandsFor   string            `json:"standsFor"`
	Properties  map[string]*Items `json:"properties"`
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
	f, err := os.Create("schemas.html")
	if err != nil {
		sklog.Fatalf("Failed to open output file: %s", err)
	}
	defer util.Close(f)
	fmt.Fprintf(f, "<html>\n <style> section { margin-left: 2em; }</style>  <body>\n")
	for _, k := range keys {
		fmt.Fprintf(f, "<h1>Schema: %s</h1>", k)
		dumpSchema(f, allSchemas[k])
	}
	fmt.Fprintf(f, "  </body>\n</html>\n")
}

func dumpExactly(w io.Writer, s *JSONSchema) {
	fmt.Fprint(w, "<section>\n")
	fmt.Fprintf(w, "<h2>Title: %s - %s</h2>\n", s.Title, s.Description)

	if len(s.Properties) > 0 {
		props := []string{}
		for k, _ := range s.Properties {
			props = append(props, k)
		}
		sort.Strings(props)

		fmt.Fprint(w, "<section>\n")
		fmt.Fprint(w, "<h3>Properties</h3>\n")
		for _, k := range props {
			p := s.Properties[k]
			fmt.Fprintf(w, "<h4>%s</h4>\n", k)
			dumpSchema(w, p)
		}
		fmt.Fprint(w, "</section>\n")
	}
	fmt.Fprint(w, "</section>\n")
}

func dumpOneOf(w io.Writer, s []*JSONSchema) {
	fmt.Fprint(w, "<section>\n")
	fmt.Fprint(w, "<h2>One Of</h2>\n")
	fmt.Fprint(w, "</section>\n")
}

func dumpSchema(w io.Writer, schema *Items) {
	if schema.Plurality == "exactly" {
		dumpExactly(w, schema.Schemas[0])
	} else if schema.Plurality == "oneOf" {
		dumpOneOf(w, schema.Schemas)
	} else {
		// Do nothing.
	}
}

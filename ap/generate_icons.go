package main

// Generates some icon/*.js files using the SVGs from the
// material-design-icons npm package. It requires no command line args.

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

const SVG_SRC = "./node_modules/material-design-icons/%s/svg/production/"
const SVG_DEST = "./elements-sk/icon/%s-icon-sk.js"

const DEMO_HTML_PATH = "./pages/icon-sk.html"
const DEMO_JS_PATH = "./pages/icon-sk.js"

var SVG_FOLDERS = []string{"action", "alert", "av", "communication", "content",
	"device", "editor", "file", "hardware", "image", "maps", "navigation",
	"notification", "places", "social", "toggle"}

var SVG_24PX_FILE = regexp.MustCompile(`(ic_)?(?P<name>.+)_24px.svg`)
var SVG_CONTENT = regexp.MustCompile(`<svg .+?>(?P<content>.+)</svg>`)

func main() {
	generatedMap := map[string][]string{}
	generated := map[string]bool{}

	fmt.Println("Generating...")
	for _, f := range SVG_FOLDERS {
		fmt.Printf("from %s: ", f)
		d := fmt.Sprintf(SVG_SRC, f)
		fmt.Println(d)
		svgs, err := ioutil.ReadDir(d)
		if err != nil {
			fmt.Printf(`Error reading from directory %s
Did you run 'npm install' first?
%s`, d, err)
			return
		}
		generatedMap[f] = []string{}
		for _, s := range svgs {
			if match := SVG_24PX_FILE.FindStringSubmatch(s.Name()); match != nil {
				file := filepath.Join(d, s.Name())
				name := sanitizeName(match[2])

				err := createIconFile(name, file)
				if err != nil {
					fmt.Printf("Error making icon file from %s: %s\n", file, err)
					return
				}
				fmt.Printf("%s ", name)
				generatedMap[f] = append(generatedMap[f], name)
				generated[name] = true
			}
		}
		fmt.Println()
	}

	h, err := os.Create(DEMO_HTML_PATH)
	if err != nil {
		fmt.Printf("cannot make demo html: %s\n", err)
		return
	}
	if err = DEMO_PAGE_HTML_TEMPLATE.Execute(h, htmlStruct{Icons: generatedMap}); err != nil {
		fmt.Printf("HTML template error %s\n", err)
	}

	j, err := os.Create(DEMO_JS_PATH)
	if err != nil {
		fmt.Printf("cannot make demo js: %s\n", err)
		return
	}

	names := []string{}
	for k := range generated {
		names = append(names, k)
	}
	sort.Strings(names)
	if err = DEMO_PAGE_JS_TEMPLATE.Execute(j, jsStruct{Names: names}); err != nil {
		fmt.Printf("JS template error %s\n", err)
	}
}

func sanitizeName(s string) string {
	s = strings.Replace(s, "_", "-", -1)
	if strings.HasPrefix(s, "3d") {
		s = strings.Replace(s, "3d", "three-d", -1)
	}
	return s
}

func createIconFile(name, srcPath string) error {
	// read in file
	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if match := SVG_CONTENT.FindStringSubmatch(string(data)); match != nil {
		// match[1] has the svg content
		created := fmt.Sprintf(SVG_DEST, name)
		if output, err := os.Create(created); err != nil {
			return err
		} else {
			err = ICON_SK_TEMPLATE.Execute(output, iconStruct{
				Name:    name,
				Content: match[1],
			})
			if err != nil {
				return fmt.Errorf("Could not write template: %s", err)
			}
		}
	} else {
		return fmt.Errorf("Could not parse svg info from %s", srcPath)
	}
	return nil
}

type iconStruct struct {
	Name    string
	Content string
}

var ICON_SK_TEMPLATE = template.Must(template.New("icon-sk").Parse(`/* This is a generated file!
 * SVG path data from https://github.com/google/material-design-icons used
 * under an Apache 2.0 license.
 */
import './icon-sk.css';
import { IconSk } from './base';

window.customElements.define('{{.Name}}-icon-sk', class extends IconSk {
  static get _svg() { return '{{ .Content }}'; }
});`))

type htmlStruct struct {
	Icons map[string][]string // maps category -> examples where examples are just the prefix.
}

var DEMO_PAGE_HTML_TEMPLATE = template.Must(template.New("icon-demo-html").Parse(`<!-- This is a generated file! -->
<!DOCTYPE html>
<title>icons-sk demo</title>
<meta charset="utf-8" />
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<script type="text/javascript" charset="utf-8">
  // This bit of script loads the custom elements v1 polyfill, but only if required.
  if (!window.customElements) {
    var s = document.createElement('script');
    s.src = 'https://cdnjs.cloudflare.com/ajax/libs/custom-elements/1.1.1/custom-elements.min.js';
    document.write(s.outerHTML);
  }
</script>
<style>
  .icon-label {
    display: inline-block;
    padding: 1em;
    text-align: center;
    margin: 2px 0;
  }

  figure {
    fill: #777;
  }

  .icon-label > * {
    margin-top: 0.6em;
    display: block;
    text-align: center;
    color: #040;
    font-size: 90%;
  }

  .icon-label > :first-child {
    min-height: 24px;
    min-width: 24px;
  }

  figcaption.label {
    display: block;
  }
</style>
<h1>icon-sk demo page</h1>
All icons have -icon-sk as a suffix, that is
<pre>account-balance -> account-balance-icon-sk</pre>
{{ range $category, $icons := .Icons}}
<h2> {{ $category }} </h2>
{{ range $icons }}
	<figure class=icon-label>
		<{{.}}-icon-sk title="{{.}}-icon-sk"></{{.}}-icon-sk>
		<figcaption class=label>{{.}}</figcaption>
	</figure>
{{end}}
{{end}}
`))

type jsStruct struct {
	Names []string
}

var DEMO_PAGE_JS_TEMPLATE = template.Must(template.New("icon-demo-js").Parse(`/* This is a generated file! */
{{range .Names}}import 'elements-sk/icon/{{.}}-icon-sk';
{{end}}`))

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

const SVG_SRC = "./node_modules/material-design-icons/%s/svg/production/"
const SVG_DEST = "./elements-sk/icon-sk/%s-icon-sk.js"

var SVG_FOLDERS = []string{"action"}

var SVG_24PX = regexp.MustCompile("(ic_)?(?P<name>.+)_24px.svg")
var PATH_D = regexp.MustCompile(`<path d="([^"]+?)"/>`)

func main() {
	for _, f := range SVG_FOLDERS {
		d := fmt.Sprintf(SVG_SRC, f)
		fmt.Println(d)
		svgs, err := ioutil.ReadDir(d)
		if err != nil {
			fmt.Printf(`Error reading from directory %s
Did you run 'npm install' first?
%s`, d, err)
			return
		}
		for _, s := range svgs {
			if match := SVG_24PX.FindStringSubmatch(s.Name()); match != nil {
				file := filepath.Join(d, s.Name())
				fmt.Printf("Saw file %s - %s\n", file, match[2])
				err := createIconFile(underscoresToDashes(match[2]), file)
				if err != nil {
					fmt.Println("Error making icon file from %s: %s", file, err)
					return
				}
				break
			}

		}
	}

	// Generate demo file with all icons
}

func underscoresToDashes(s string) string {
	return strings.Replace(s, "_", "-", -1)
}

func createIconFile(name, srcPath string) error {
	// read in file
	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if match := PATH_D.FindStringSubmatch(string(data)); match != nil {
		// match[1] has the path data
		// writeOutTemplate
		if output, err := os.Create(fmt.Sprintf(SVG_DEST, name)); err != nil {
			return err
		} else {
			err = ICON_SK_TEMPLATE.Execute(output, iconStruct{
				Name: name,
				Path: match[1],
			})
			if err != nil {
				return fmt.Errorf("Could not write template: %s", err)
			}
		}
	} else {
		return fmt.Errorf("Cannot read SVG path 'd' from %s", srcPath)
	}
	return nil
}

type iconStruct struct {
	Name string
	Path string
}

var ICON_SK_TEMPLATE = template.Must(template.New("icon-sk").Parse(`/* This is a generated file!
 * SVG path data from https://github.com/google/material-design-icons used
 * under an Apache 2.0 license.
 */
import './icon-sk.css';
import { IconSk } from './base';

window.customElements.define('{{.Name}}-icon-sk', class extends IconSk {
  static get _path() { return "{{ .Path }}"; }
});
`))

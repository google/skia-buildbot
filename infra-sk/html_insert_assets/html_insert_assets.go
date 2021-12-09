// This program takes an HTML page and inserts <link> and <script> tags to load CSS and JavaScript
// files at the specified locations, with an optional "nonce" attribute with the specified value.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
)

func insertAssets(inputHTML io.Reader, jsPath, cssPath, nonce string) (string, error) {
	var output []string
	tokenizer := html.NewTokenizer(inputHTML)
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return strings.Join(output, ""), nil
			} else {
				return "", tokenizer.Err()
			}
		case html.EndTagToken:
			tagNameBytes, _ := tokenizer.TagName()
			tagName := string(tagNameBytes)
			if tagName == "head" {
				// Insert <link> tag.
				if nonce != "" {
					output = append(output, fmt.Sprintf("<link rel=\"stylesheet\" href=%q nonce=%q>", cssPath, nonce))
				} else {
					output = append(output, fmt.Sprintf("<link rel=\"stylesheet\" href=%q>", cssPath))
				}
			}
			if tagName == "body" {
				// Insert <script> tag.
				if nonce != "" {
					output = append(output, fmt.Sprintf("<script src=%q nonce=%q></script>", jsPath, nonce))
				} else {
					output = append(output, fmt.Sprintf("<script src=%q></script>", jsPath))
				}
			}
		}
		output = append(output, string(tokenizer.Raw()))
	}
}

func main() {
	html := flag.String("html", "", "Path to the HTML file to instrument.")
	js := flag.String("js", "", "Serving path of the JavaScript file to insert.")
	css := flag.String("css", "", "Serving path of the CSS file to insert.")
	nonce := flag.String("nonce", "", "Value of the nonce attribute of the inserted link/script tags. Optional.")

	flag.Parse()
	if *html == "" || *js == "" || *css == "" {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	htmlReader, err := os.Open(*html)
	if err != nil {
		panic(err)
	}

	instrumentedHTML, err := insertAssets(htmlReader, *js, *css, *nonce)
	if err != nil && err != io.EOF {
		panic(err)
	}

	fmt.Printf("%s", instrumentedHTML)
}

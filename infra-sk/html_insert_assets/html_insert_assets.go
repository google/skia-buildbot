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
	var endHeadTagFound bool
	var endBodyTagFound bool
	tokenizer := html.NewTokenizer(inputHTML)

loop:
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				break loop
			} else {
				return "", tokenizer.Err()
			}
		case html.EndTagToken:
			tagNameBytes, _ := tokenizer.TagName()
			tagName := string(tagNameBytes)
			if tagName == "head" {
				endHeadTagFound = true
				// Insert <link> tag.
				if nonce != "" {
					output = append(output, fmt.Sprintf("<link rel=\"stylesheet\" href=%q nonce=%q>", cssPath, nonce))
				} else {
					output = append(output, fmt.Sprintf("<link rel=\"stylesheet\" href=%q>", cssPath))
				}
			}
			if tagName == "body" {
				endBodyTagFound = true
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

	if !endHeadTagFound {
		return "", fmt.Errorf("no </head> tag found")
	}
	if !endBodyTagFound {
		return "", fmt.Errorf("no </body> tag found")
	}
	return strings.Join(output, ""), nil
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

	ifErrThenDie := func(err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while instrumenting %s: %s", *html, err)
			os.Exit(1)
		}
	}

	htmlReader, err := os.Open(*html)
	ifErrThenDie(err)

	instrumentedHTML, err := insertAssets(htmlReader, *js, *css, *nonce)
	ifErrThenDie(err)

	fmt.Printf("%s", instrumentedHTML)
}

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
	z := html.NewTokenizer(inputHTML)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return strings.Join(output, ""), nil
			} else {
				return "", z.Err()
			}
		case html.EndTagToken:
			tagNameBytes, _ := z.TagName()
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
		output = append(output, string(z.Raw()))
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

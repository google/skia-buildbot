package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/PuerkitoBio/goquery"
	"go.skia.org/infra/go/sklog"
)

const body = `
<!DOCTYPE html>
<html>
<head>
    <title>A Page</title>
    <meta charset="utf-8" />
        <meta http-equiv="X-UA-Compatible" content="IE=edge">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
    <details>
        <summary><img src="https://dot.skia.org/dot?foo"></src></summary>
        <pre>
        graph {
            Hello -- World
        }
        </pre>
    </details>
</body>
</html>
`

func dotToSVG(ctx context.Context, dotCode string) (string, error) {
	cmd := exec.CommandContext(ctx, "dot", "-Tsvg")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("Failed to create stdin pipe to dot: %s", err)
	}

	go func() {
		defer stdin.Close()
		_, err := io.WriteString(stdin, dotCode)
		if err != nil {
			sklog.Errorf("Failed to write to dot stdin: %s", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func main() {
	buf := bytes.NewBufferString(body)

	// Load the HTML document
	// md5 hash it
	// If there is an image in the cache for md5+url then return that else:

	// Load the referring document, parse HTML, find the img tag with the URL we
	// are currently servicing, find the nearby dot code, turn it into SVG,
	// store in the cache, and return the SVG.
	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		log.Fatal(err)
	}
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		fmt.Println(s.Attr("src"))
		dotCode := s.Parent().Parent().Find("pre").Text()
		fmt.Println(dotToSVG(context.Background(), dotCode))
	})
	// If not found then return a 404 SVG.
}

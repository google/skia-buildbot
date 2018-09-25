package main

import (
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/common"
)

func main() {
	common.Init()
	for _ = range time.Tick(time.Second) {
		resp, err := http.Get("https://leasing.skia.org")
		if err != nil {
			fmt.Printf("%s: %s\n", time.Now().UTC(), err)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			fmt.Printf("%s: Status code: %d %s\n", time.Now().UTC(), resp.StatusCode, err)
			continue
		}
	}
}

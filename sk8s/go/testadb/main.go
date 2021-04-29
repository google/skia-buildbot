package main

import (
	"context"
	"fmt"

	"go.skia.org/infra/sk8s/go/bot_config/adb"
)

func main() {
	a := adb.New()
	uptime, err := a.Uptime(context.Background())
	fmt.Println(uptime, err)
}

package main

import (
	"flag"
)

func main() {
	flag.Parse()
	Init()
	RunIngester()
}

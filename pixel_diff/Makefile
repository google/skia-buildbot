.PHONY: all
all: frontend allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: skiasilver
skiasilver: skiaversion
	go install -v ./go/skiasilver

.PHONY: skia_diff_server
skia_diff_server: skiaversion
	go install -v ./go/skia_diff_server

.PHONY: imagediff
imagediff:
	go install -v ./go/imagediff

.PHONY: netdiff
netdiff:
	go install -v ./go/netdiff

.PHONY: packages
packages:
	go build -v ./go/...

node_modules: package.json
	npm install

.PHONY: frontend
frontend:
	cd frontend && $(MAKE) web

.PHONY: allgo
allgo: skiasilver imagediff skia_diff_server

include ../webtools/webtools.mk

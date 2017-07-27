.PHONY: all
all: frontend allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: ct_pixel_diff
ct_pixel_diff: skiaversion
	go install -v ./go/ct_pixel_diff

.PHONY: ctdiffingestion
ctdiffingestion:
	go install -v ./go/ctdiffingestion

.PHONY: resultstore
resultstore:
	go install -v ./go/resultstore

.PHONY: packages
packages:
	go build -v ./go/...

node_modules: package.json
	npm install

.PHONY: frontend
frontend:
	cd frontend && $(MAKE) web

.PHONY: allgo
allgo: ct_pixel_diff ctdiffingestion resultstore

include ../webtools/webtools.mk

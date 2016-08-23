.PHONY: all
all: frontend allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: skiacorrectness
skiacorrectness: skiaversion
	go install -v ./go/skiacorrectness

.PHONY: correctness_migratedb
correctness_migratedb: skiaversion
	go install -v ./go/correctness_migratedb

.PHONY: imagediff
imagediff:
	go install -v ./go/imagediff

.PHONY: sampler
sampler:
	go install -v ./go/sampler

.PHONY: packages
packages:
	go build -v ./go/...

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -short

.PHONY: bench
bench:
	go test ./go/... -bench=. -run=NONE

.PHONY: test
test:
	go test ./go/...

node_modules: package.json
	npm install

.PHONY: frontend
frontend:
	cd frontend && $(MAKE) web

.PHONY: allgo
allgo: skiacorrectness correctness_migratedb imagediff sampler

include ../webtools/webtools.mk

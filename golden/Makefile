.PHONY: skiacorrectness
skiacorrectness:
	go install -v ./go/skiacorrectness

.PHONY: correctness_migratedb
correctness_migratedb:
	go install -v ./go/correctness_migratedb


.PHONY: packages
packages:
	go build -v ./go/...

.PHONY: test
test:
	go test ./go/...

.PHONY: all
all: skiacorrectness correctness_migratedb

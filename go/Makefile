
.PHONY: migratedb
migratedb:
	go install -v ./migratedb

.PHONY: packages
packages:
	go install -v ./...

.PHONY: test
test:
	go test ./...

.PHONY: all
all: migratedb packages

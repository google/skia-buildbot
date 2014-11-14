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

# Build the web resources.
web: node_modules Gruntfile.js bower.json
	./node_modules/.bin/grunt --no-color

node_modules: package.json
	npm install

.PHONY: all
all: skiacorrectness correctness_migratedb web

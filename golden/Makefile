.PHONY: skiacorrectness
skiacorrectness:
	go install -v ./go/skiacorrectness

.PHONY: packages
packages:
	go build -v ./go/...

.PHONY: test
test:
	go test ./go/...

.PHONY: all
all: skiacorrectness

build:
	GOPATH=$(GOPATH):`pwd` go build -o perf server

test:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace

verbosetests:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace -v

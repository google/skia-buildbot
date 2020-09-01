

default: rice-box.go
	go install .

# The command line tool that embeds plain files into Go code.
${GOPATH}/bin/rice:
	go get github.com/GeertJohan/go.rice/rice

rice-box.go: $(wildcard ./templates/*) ${GOPATH}/bin/rice
	rice embed-go

example: default
	-rm -rf modules
	mkdir modules
	new_element example-control-sk
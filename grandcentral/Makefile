include ../go/skiaversion/skiaversion.mk

all: grandcentral

grandcentral: skiaversion
	go install -v ./go/grandcentral

testgo: skiaversion
	go test ./go/... -v


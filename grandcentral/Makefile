include ../go/skiaversion/skiaversion.mk

all: grandcentral event_viewer event_gen

grandcentral: skiaversion
	go install -v ./go/grandcentral

event_viewer: skiaversion
	go install -v ./go/event_viewer


event_gen: skiaversion
	go install -v ./go/event_gen

testgo: skiaversion
	go test ./go/... -v

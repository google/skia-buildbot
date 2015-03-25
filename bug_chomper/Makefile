include ../go/skiaversion/skiaversion.mk

all: bug_chomper

bug_chomper: skiaversion
	go install -v ./go/bug_chomper

testgo: skiaversion
	go test ./go/... -v

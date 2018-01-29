include ../go/skiaversion/skiaversion.mk

.PHONY: all
all: flakiness

.PHONY: flakiness
flakiness: skiaversion
	go install -v ./go/flakiness

.PHONY: testgo
testgo: skiaversion
	go test ./go/... -v --small --medium

.PHONY: testgo_all
testgo_all: skiaversion
	go test ./go/... -v --small --medium --large

.PHONY: push
push: all
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli flakiness skia-flakiness

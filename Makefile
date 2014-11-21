testgo:
	go test ./go/...

.PHONY: sharedgo
sharedgo:
	cd go && $(MAKE) all

.PHONY: golden
golden:
	cd golden && $(MAKE) all

.PHONY: perf
perf:
	cd perf && $(MAKE) all

.PHONY: monitoring
monitoring:
	cd monitoring && $(MAKE) all

.PHONY: all
all: golden monitoring perf sharedgo

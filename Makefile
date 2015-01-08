testgo:
	go test -test.short -i ./go/...

.PHONY: sharedgo
sharedgo:
	cd go && $(MAKE) all

.PHONY: golden
golden:
	cd golden && $(MAKE) all

.PHONY: perf
perf:
	cd perf && $(MAKE) all

.PHONY: alertserver
alertserver:
	cd alertserver && $(MAKE) all

.PHONY: datahopper
datahopper:
	cd  datahopper && $(MAKE) default

.PHONY: logserver
logserver:
	cd  logserver && $(MAKE) default

.PHONY: ct
ct:
	cd ct && $(MAKE) all

.PHONY: grains
grains:
	cd grains && $(MAKE) default

.PHONY: push
push:
	cd push && $(MAKE) default

.PHONY: all
all: golden perf sharedgo alertserver datahopper logserver ct

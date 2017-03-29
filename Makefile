testgo:
	go test -test.short ./go/...

.PHONY: testjs
testjs:
	cd js && $(MAKE) test

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

.PHONY: build_scheduler
build_scheduler:
	cd build_scheduler && $(MAKE) all

.PHONY: datahopper
datahopper:
	cd datahopper && $(MAKE) all

.PHONY: datahopper_internal
datahopper_internal:
	cd datahopper_internal && $(MAKE) default

.PHONY: logserver
logserver:
	cd  logserver && $(MAKE) default

.PHONY: ct
ct:
	cd ct && $(MAKE) all

.PHONY: ctfe
ctfe:
	cd ct && $(MAKE) ctfe

.PHONY: push
push:
	cd push && $(MAKE) default

.PHONY: status
status:
	cd status && $(MAKE) all

.PHONY: fuzzer
fuzzer:
	cd fuzzer && $(MAKE) all

.PHONY: skolo
skolo:
	cd skolo && $(MAKE) all

.PHONY: all
all: alertserver build_scheduler datahopper datahopper_internal golden perf sharedgo logserver ct ctfe status tags

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

.PHONY: buildall
buildall:
	go build ./...

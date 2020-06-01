testgo:
	go test -test.short ./go/...

.PHONY: testjs
testjs:
	cd js && $(MAKE) test

.PHONY: sharedgo
sharedgo:
	cd go && $(MAKE) all

#.PHONY: golden
#golden:
#	cd golden && $(MAKE) all

.PHONY: perf
perf:
	cd perf && $(MAKE) all

.PHONY: autoroll
autoroll:
	cd autoroll && $(MAKE) all

.PHONY: cq_watcher
cq_watcher:
	cd cq_watcher && $(MAKE) default

.PHONY: datahopper
datahopper:
	cd datahopper && $(MAKE) all

.PHONY: ct
ct:
	cd ct && $(MAKE) all

.PHONY: ctfe
ctfe:
	cd ct && $(MAKE) ctfe

.PHONY: infra-sk
infra-sk:
	cd infra-sk && $(MAKE) all

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

.PHONY: task_scheduler
task_scheduler:
	cd task_scheduler && $(MAKE) all

.PHONY: all
all: infra-sk autoroll datahopper perf sharedgo ct ctfe cq_watcher status task_scheduler

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

.PHONY: buildall
buildall:
	go build ./...

.PHONY: puppeteer-tests
puppeteer-tests:
	docker run --interactive --rm \
		--mount type=bind,source=`pwd`,target=/src \
		--mount type=bind,source=`pwd`/puppeteer-tests/output,target=/out \
		gcr.io/skia-public/puppeteer-tests:latest \
		/src/puppeteer-tests/docker/run-tests.sh

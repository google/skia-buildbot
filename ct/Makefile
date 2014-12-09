
# Master scripts.
.PHONY: run_command
run_command:
	go install -v ./go/master_scripts/run_command/

# Worker scripts.
.PHONY: create_pagesets
create_pagesets:
	go install -v ./go/worker_scripts/create_pagesets/

.PHONY: capture_archives
capture_archives:
	go install -v ./go/worker_scripts/capture_archives/

.PHONY: run_lua
run_lua:
	go install -v ./go/worker_scripts/run_lua/

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -v -test.short

.PHONY: test
test:
	go test ./go/...

.PHONY: master_scripts
master_scripts: run_command

.PHONY: worker_scripts
worker_scripts: create_pagesets capture_archives run_lua

.PHONY: all
all: run_command create_pagesets

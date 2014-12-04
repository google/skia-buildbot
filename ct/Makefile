
# Master scripts.
.PHONY: run_command
run_command:
	go install -v ./go/master_scripts/run_command/

# Worker scripts.
.PHONY: create_pagesets
create_pagesets:
	go install -v ./go/worker_scripts/create_pagesets/

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
worker_scripts: create_pagesets

.PHONY: all
all: run_command create_pagesets

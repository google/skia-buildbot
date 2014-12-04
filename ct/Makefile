
.PHONY: run_command
run_command:
	go install -v ./go/master_scripts/run_command/

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -v -test.short

.PHONY: test
test:
	go test ./go/...

.PHONY: all
all: run_command

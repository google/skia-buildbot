
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

.PHONY: run_benchmark
run_benchmark:
	go install -v ./go/worker_scripts/run_benchmark/

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
worker_scripts: create_pagesets capture_archives run_lua run_benchmark

.PHONY: all
all: run_command create_pagesets

.PHONY: e2e_tests
e2e_tests:
	@echo "Running create_pagesets"
	go run go/worker_scripts/create_pagesets/main.go -logtostderr --worker_num=1 --pageset_type=Dummy100
	@echo "Completed create_pagests. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo
	
	@echo "Running capture_archives"
	go run go/worker_scripts/capture_archives/main.go -logtostderr --worker_num=1 --pageset_type=Dummy100 --chromium_build=38517cc-d7e49ce -logtostderr
	@echo "Completed capture_archives. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo
	 
	@echo "Running skpicture_printer"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy100 --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=skpicture_printer --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed skpicture_printer. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo
	
	@echo "Running smoothness"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy100 --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=smoothness --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed smoothness. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo
	
	@echo "Running rasterize_and_record_micro"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy100 --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=rasterize_and_record_micro --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed rasterize_and_record_micro. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo
	
	@echo "Running repaint"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy100 --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=repaint --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed repaint. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

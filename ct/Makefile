
# Master scripts.
.PHONY: run_command
run_command:
	go install -v ./go/master_scripts/run_command/

.PHONY: check_workers_health
check_workers_health:
	go install -v ./go/master_scripts/check_workers_health/

.PHONY: build_chromium
build_chromium:
	go install -v ./go/master_scripts/build_chromium/

.PHONY: create_pagesets_on_workers
create_pagesets_on_workers:
	go install -v ./go/master_scripts/create_pagesets_on_workers/

.PHONY: capture_archives_on_workers
capture_archives_on_workers:
	go install -v ./go/master_scripts/capture_archives_on_workers/

.PHONY: run_lua_on_workers
run_lua_on_workers:
	go install -v ./go/master_scripts/run_lua_on_workers/

.PHONY: run_benchmark_on_workers
run_benchmark_on_workers:
	go install -v ./go/master_scripts/run_benchmark_on_workers/

.PHONY: run_chromium_perf_on_workers
run_chromium_perf_on_workers:
	go install -v ./go/master_scripts/run_chromium_perf_on_workers/

.PHONY: run_skia_correctness_on_workers
run_skia_correctness_on_workers:
	go install -v ./go/master_scripts/run_skia_correctness_on_workers/

.PHONY: fix_archives_on_workers
fix_archives_on_workers:
	go install -v ./go/master_scripts/fix_archives_on_workers/

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

.PHONY: run_skia_correctness
run_skia_correctness:
	go install -v ./go/worker_scripts/run_skia_correctness/

.PHONY: fix_archives
fix_archives:
	go install -v ./go/worker_scripts/fix_archives/

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -v -test.short

.PHONY: test
test:
	go test ./go/...

.PHONY: master_scripts
master_scripts: run_command check_workers_health build_chromium create_pagesets_on_workers capture_archives_on_workers run_lua_on_workers run_benchmark_on_workers run_chromium_perf_on_workers run_skia_correctness_on_workers fix_archives_on_workers

.PHONY: worker_scripts
worker_scripts: create_pagesets capture_archives run_lua run_benchmark run_skia_correctness fix_archives

.PHONY: all
all: master_scripts worker_scripts testgo

.PHONY: e2e_tests
e2e_tests:
	@echo "Running create_pagesets"
	go run go/worker_scripts/create_pagesets/main.go -logtostderr --worker_num=1 --pageset_type=Dummy1k
	@echo "Completed create_pagests. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running capture_archives"
	go run go/worker_scripts/capture_archives/main.go -logtostderr --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce -logtostderr
	@echo "Completed capture_archives. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running skpicture_printer"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=skpicture_printer --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed skpicture_printer. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running smoothness"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=smoothness --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed smoothness. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running rasterize_and_record_micro"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=rasterize_and_record_micro --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed rasterize_and_record_micro. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running repaint"
	go run go/worker_scripts/run_benchmark/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce -logtostderr --run_id=rmistry123 --benchmark_name=repaint --browser_extra_args="--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting" --repeat_benchmark=2 --target_platform=Linux
	@echo "Completed repaint. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running skia_correctness"
	go run go/worker_scripts/run_skia_correctness/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=c5e7bbe-4fc48af --run_id=rmistry123 --render_pictures_args="--config 8888 --viewport 4000 4000 --mode simple" --gpu_nopatch_run=false --gpu_withpatch_run=true --logtostderr
	@echo "Completed skia_correctness. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

	@echo "Running lua scripts"
	go run go/worker_scripts/run_lua/main.go --worker_num=1 --pageset_type=Dummy1k --chromium_build=38517cc-d7e49ce --run_id=rmistry123 --logtostderr
	@echo "Completed lua scripts. Sleeping for 15 seconds and continuing."
	sleep 15
	@echo
	@echo

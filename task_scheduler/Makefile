VULCANIZE1=true

BOWER_DIR=third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
	third_party/bower_components/webcomponentsjs/webcomponents.min.js \
	$(BOWER_DIR)/d3/d3.js \
	../res/js/common.js


include ../go/skiaversion/skiaversion.mk
include ../kube/kube.mk

.PHONY: all
all: task_scheduler_kube validate_db

.PHONY: task_scheduler_kube
task_scheduler_kube: core_js elements_html skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task_scheduler

.PHONY: testgo
testgo: skiaversion
	go test ./go/... -v --small --medium

.PHONY: testgo_all
testgo_all: skiaversion
	go test ./go/... -v --small --medium --large

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: task_scheduler_debug
task_scheduler_debug: debug skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task_scheduler

.PHONY: release_kube
release_kube: task_scheduler_kube build_base_cipd_release
	./build_docker_release

.PHONY: push_kube
push_kube: release_kube skia-public
	pushk --message="$(MESSAGE)" task-scheduler

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db

include ../webtools/webtools.mk

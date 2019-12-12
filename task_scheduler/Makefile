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
all: task-scheduler-be task-scheduler-fe validate_db

.PHONY: task-scheduler-be
task-scheduler-be: skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-be

.PHONY: task-scheduler-fe
task-scheduler-fe: core_js elements_html skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-fe

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: task-scheduler-fe-debug
task-scheduler-fe-debug: debug skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-fe

.PHONY: release-be
release-be: task-scheduler-be build_base_cipd_release
	./build_be_release

.PHONY: release-fe
release-fe: task-scheduler-fe
	./build_fe_release

.PHONY: push-be
push-be: release-be
	pushk --message="$(MESSAGE)" task-scheduler-be

.PHONY: push-fe
push-fe: release-fe
	pushk --message="$(MESSAGE)" task-scheduler-fe

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db

include ../webtools/webtools.mk
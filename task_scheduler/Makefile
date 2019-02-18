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

.PHONY: all
all: task_scheduler validate_db

.PHONY: task_scheduler
task_scheduler: core_js elements_html skiaversion
	go install -v ./go/task_scheduler

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
	go install -v ./go/task_scheduler

.PHONY: push
push: all
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli task-scheduler skia-task-scheduler

.PHONY: push_internal
push_internal: all
	./build_internal_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli task-scheduler-internal skia-task-scheduler-internal

.PHONY: push_staging
push_staging: all
	./build_staging_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli task-scheduler-staging skia-task-scheduler-staging

.PHONY: push_backup
push_backup: all
	./build_backup_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli task-scheduler-db-backup skia-task-scheduler

.PHONY: vm_create_prod
vm_create_prod:
	go run ./vm.go --alsologtostderr --create --ignore-exists --instance=prod

.PHONY: vm_delete_prod
vm_delete_prod:
	go run ./vm.go --alsologtostderr --delete --ignore-exists --instance=prod

.PHONY: vm_create_internal
vm_create_internal:
	go run ./vm.go --alsologtostderr --create --ignore-exists --instance=internal

.PHONY: vm_delete_internal
vm_delete_internal:
	go run ./vm.go --alsologtostderr --delete --ignore-exists --instance=internal

.PHONY: vm_create_staging
vm_create_staging:
	go run ./vm.go --alsologtostderr --create --ignore-exists --instance=staging

.PHONY: vm_delete_staging
vm_delete_staging:
	go run ./vm.go --alsologtostderr --delete --ignore-exists --instance=staging

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db

include ../webtools/webtools.mk

VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

include ../go/skiaversion/skiaversion.mk
include ../kube/kube.mk
include ../make/clusters.mk
include ../webtools/webtools.mk

.PHONY: autoroll-fe
autoroll-fe: core_js elements_html skiaversion
	ln -sf ../../res res/imp/res
	GOOS=linux go install -a ./go/autoroll-fe

.PHONY: autoroll-be
autoroll-be: skiaversion
	GOOS=linux go install -a ./go/autoroll-be

.PHONY: autoroll-google3
autoroll-google3: skiaversion
	GOOS=linux go install -a ./go/autoroll-google3

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

ROLLER_INTERNAL_CONFIG_DIR = "/tmp/skia-autoroll-internal-config"
USE_INTERNAL_CONFIG_DIR_PROMPT ?= $$(bash -c 'read -s -p "The checkout in $(ROLLER_INTERNAL_CONFIG_DIR) is dirty. Would you still like to use it? (y/n): " ans; echo $$ans')

setup-internal-config-repo:
	if [[ ! -d $(ROLLER_INTERNAL_CONFIG_DIR) ]]; then \
		git clone https://skia.googlesource.com/skia-autoroll-internal-config.git $(ROLLER_INTERNAL_CONFIG_DIR); \
	else \
		git -C $(ROLLER_INTERNAL_CONFIG_DIR) fetch && \
		git -C $(ROLLER_INTERNAL_CONFIG_DIR) diff --quiet origin/master || \
		if [ $$? -ne 0 ]; then \
			if [[ "$(USE_INTERNAL_CONFIG_DIR_PROMPT)" != "y" ]]; then \
				echo "Exiting"; exit 1; \
			fi; \
		fi; \
	fi

# These targets build kubernetes releases.
.PHONY: release-be
release-be: autoroll-be
	./build_backend_release

.PHONY: release-fe
release-fe: autoroll-fe
	./build_frontend_release

.PHONY: release-google3
release-google3: autoroll-google3 setup-internal-config-repo
	CONFIG_DIR="$(ROLLER_INTERNAL_CONFIG_DIR)" ./build_google3_release "$(MESSAGE)"

.PHONY: release-all
release-all: release-fe release-be release-google3

# push-helper is used by the below targets to apply k8s configurations.
.PHONY: push-helper
push-helper: kube-conf-gen deployment-dirs
	go install -v ./go/push-helper

# These targets apply config changes but not new builds.
.PHONY: apply-configs-external
apply-configs-external: push-helper
	push-helper --apply --external

.PHONY: apply-configs-internal
apply-configs-internal: push-helper setup-internal-config-repo
	push-helper --apply --internal

.PHONY: apply-configs-all
apply-configs-all: apply-configs-external apply-configs-internal

# These targets apply the latest build, in addition to any config changes.
.PHONY: apply-build-be-regex
apply-build-be-regex: release-be push-helper
	push-helper --apply --update-roller "$(REGEX)"

.PHONY: apply-build-be-external
apply-build-be-external: release-be push-helper
	push-helper --apply --external --update-be-image

.PHONY: apply-build-be-internal
apply-build-be-internal: release-be push-helper setup-internal-config-repo
	push-helper --apply --external --update-be-image

.PHONY: apply-build-be-all
apply-build-be-all: apply-build-be-external apply-build-be-internal

.PHONY: apply-build-fe-external
apply-build-fe-external: release-fe push-helper
	push-helper --apply --external --update-fe-image

.PHONY: apply-build-fe-internal
apply-build-fe-internal: release-fe push-helper setup-internal-config-repo
	push-helper --apply --external --update-fe-image

.PHONY: apply-build-fe-all
apply-build-fe-all: apply-build-fe-external apply-build-fe-internal

# These targets push config changes but not new builds.
.PHONY: push-configs-external
push-configs-external: push-helper
	push-helper --apply --external --commit-with-msg="$(MESSAGE)"

.PHONY: push-configs-internal
push-configs-internal: push-helper setup-internal-config-repo
	push-helper --apply --internal --commit-with-msg="$(MESSAGE)"

.PHONY: push-configs-all
push-configs-all: push-configs-external push-configs-internal

# These targets push the latest build, in addition to any config changes.
.PHONY: push-build-be-regex
push-build-be-regex: release-be push-helper
	push-helper --apply --update-roller "$(REGEX)" --commit-with-msg="$(MESSAGE)"

.PHONY: push-build-be-external
push-build-be-external: release-be push-helper
	push-helper --apply --external --update-be-image --commit-with-msg="$(MESSAGE)"

.PHONY: push-build-be-internal
push-build-be-internal: release-be push-helper setup-internal-config-repo
	push-helper --apply --external --update-be-image --commit-with-msg="$(MESSAGE)"

.PHONY: push-build-be-all
push-build-be-all: push-build-be-external push-build-be-internal

.PHONY: push-build-fe-external
push-build-fe-external: release-fe push-helper
	push-helper --apply --external --update-fe-image --commit-with-msg="$(MESSAGE)"

.PHONY: push-build-fe-internal
push-build-fe-internal: release-fe push-helper setup-internal-config-repo
	push-helper --apply --external --update-fe-image --commit-with-msg="$(MESSAGE)"

.PHONY: push-build-fe-all
push-build-fe-all: push-build-fe-external push-build-fe-internal

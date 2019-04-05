VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

include ../go/skiaversion/skiaversion.mk

all: autoroll-fe autoroll-be-kube autoroll-google3

autoroll-fe: core_js elements_html skiaversion
	ln -sf ../../res res/imp/res
	GOOS=linux go install -a ./go/autoroll-fe

autoroll-be-kube: skiaversion
	GOOS=linux go install -a ./go/autoroll-be-kube

autoroll-google3: skiaversion
	GOOS=linux go install -a ./go/autoroll-google3

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

testgo: skiaversion
	go test ./go/... -v

include ../webtools/webtools.mk

release-fe: autoroll-fe setup-internal-config-repo
	./build_frontend_release

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

# Parse the config files, compile lists of roller configs.
ROLLER_CONFIGS_KUBE_INTERNAL=$(wildcard /tmp/skia-autoroll-internal-config/*.json)
ROLLER_CONFIGS_KUBE_EXTERNAL=$(wildcard config/*.json)

release-be-kube: autoroll-be-kube
	CONFIG_DIR="$(CONFIG_DIR)" ./build_docker_backend_release "$(CONFIG_FILE)" "$(MESSAGE)"

release-google3: autoroll-google3 setup-internal-config-repo
	CONFIG_DIR="$(ROLLER_INTERNAL_CONFIG_DIR)" ./build_google3_release "$(MESSAGE)"

release-all: release-fe release-be-all release-google3

include ../kube/kube.mk

KUBE_CONF_GEN_INTERNAL=kube-conf-gen -t ./go/autoroll-be-kube/autoroll-be.yaml.template \
	-c "$(CONFIG_FILE)" \
	-o $(SKIA_CORP_CONFIG_DIR)/autoroll-be-$(basename $(notdir $(CONFIG_FILE))).yaml \
	--extra configFile:$(notdir $(CONFIG_FILE))

KUBE_CONF_GEN_EXTERNAL=kube-conf-gen -t ./go/autoroll-be-kube/autoroll-be.yaml.template \
	-c "$(CONFIG_FILE)" \
	-o $(SKIA_PUBLIC_CONFIG_DIR)/autoroll-be-$(basename $(notdir $(CONFIG_FILE))).yaml \
	--extra configFile:$(notdir $(CONFIG_FILE))

CONFIG_IS_INTERNAL=$(shell python -c "import json,sys; print ('1' if json.load(open(sys.argv[1]))['isInternal'] else '0')" $(CONFIG_FILE))

push-be-kube: release-be-kube kube-conf-gen pushk deployment-dirs $(CONFIG_FILE)
	if [ "$(CONFIG_IS_INTERNAL)" == "1" ]; then $(KUBE_CONF_GEN_INTERNAL); else $(KUBE_CONF_GEN_EXTERNAL); fi
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster="$(shell python -c "import json,sys; print ('skia-corp' if json.load(open(sys.argv[1])).get('isInternal') else 'skia-public')" $(CONFIG_FILE))" autoroll-be

push-be-kube-internal: release-be-kube kube-conf-gen pushk deployment-dirs setup-internal-config-repo $(SKIA_CORP_CONFIG_DIR) $(ROLLER_CONFIGS_KUBE_INTERNAL)
	$(foreach CONFIG_FILE, $(ROLLER_CONFIGS_KUBE_INTERNAL), $(KUBE_CONF_GEN_INTERNAL);)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-corp autoroll-be

push-be-kube-external: release-be-kube kube-conf-gen pushk deployment-dirs $(SKIA_PUBLIC_CONFIG_DIR) $(ROLLER_CONFIGS_KUBE_EXTERNAL)
	$(foreach CONFIG_FILE, $(ROLLER_CONFIGS_KUBE_EXTERNAL), $(KUBE_CONF_GEN_EXTERNAL);)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-public autoroll-be

push-be-kube-all: push-be-kube-internal push-be-kube-external

push-fe: release-fe
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-corp autoroll-fe-internal
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-public autoroll-fe

push-google3: release-google3 pushk deployment-dirs $(SKIA_PUBLIC_CONFIG_DIR)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-public autoroll-google3

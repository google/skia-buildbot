VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

include ../go/skiaversion/skiaversion.mk

all: autoroll-fe autoroll-be autoroll-google3

autoroll-fe: core_js elements_html skiaversion
	ln -sf ../../res res/imp/res
	CGO_ENABLED=0 GOOS=linux go install -a ./go/autoroll-fe

autoroll-be: skiaversion
	go install -v ./go/autoroll-be

autoroll-be-kube: skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/autoroll-be-kube

autoroll-google3: skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/autoroll-google3

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

testgo: skiaversion
	go test ./go/... -v

include ../webtools/webtools.mk

release-fe: autoroll-fe
	./build_frontend_release

# Parse the config files, compile lists of roller configs.
ROLLER_CONFIGS=$(wildcard config/*.json)
ROLLER_CONFIGS_GCE=$(shell python -c "import json,sys; print ' '.join([f for f in sys.argv[1:] if json.load(open(f)).get('kubernetes') is None])" $(ROLLER_CONFIGS))
ROLLER_CONFIGS_KUBE=$(shell python -c "import json,sys; print ' '.join([f for f in sys.argv[1:] if json.load(open(f)).get('kubernetes') is not None])" $(ROLLER_CONFIGS))
ROLLER_CONFIGS_KUBE_INTERNAL=$(shell python -c "import json,sys; c = [(f, json.load(open(f))) for f in sys.argv[1:]]; print ' '.join([name for name, f in c if f.get(u'kubernetes') is not None and f[u'isInternal']])" $(ROLLER_CONFIGS))
ROLLER_CONFIGS_KUBE_EXTERNAL=$(shell python -c "import json,sys; c = [(f, json.load(open(f))) for f in sys.argv[1:]]; print ' '.join([name for name, f in c if f.get(u'kubernetes') is not None and not f[u'isInternal']])" $(ROLLER_CONFIGS))

release-be: autoroll-be
	./deprecated_build_backend_release "$(CONFIG_FILE)" "$(MESSAGE)"

release-be-all: autoroll-be $(ROLLER_CONFIGS_GCE)
	$(foreach cfg, $(ROLLER_CONFIGS_GCE), ./deprecated_build_backend_release "$(cfg)" "$(MESSAGE)";)

release-be-kube: autoroll-be-kube
	./build_docker_backend_release "$(CONFIG_FILE)" "$(MESSAGE)"

release-google3: autoroll-google3
	./build_google3_release "$(MESSAGE)"

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

push-be-kube-internal: release-be-kube kube-conf-gen pushk deployment-dirs $(SKIA_CORP_CONFIG_DIR) $(ROLLER_CONFIGS_KUBE_INTERNAL)
	$(foreach CONFIG_FILE, $(ROLLER_CONFIGS_KUBE_INTERNAL), $(KUBE_CONF_GEN_INTERNAL);)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-corp autoroll-be

push-be-kube-external: release-be-kube kube-conf-gen pushk deployment-dirs $(SKIA_PUBLIC_CONFIG_DIR) $(ROLLER_CONFIGS_KUBE_EXTERNAL)
	$(foreach CONFIG_FILE, $(ROLLER_CONFIGS_KUBE_EXTERNAL), $(KUBE_CONF_GEN_EXTERNAL);)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-public autoroll-be

push-be-kube-all: push-be-kube-internal push-be-kube-external

push-google3: release-google3 pushk deployment-dirs $(SKIA_PUBLIC_CONFIG_DIR)
	pushk --ignore-dirty --message="$(MESSAGE)" --cluster=skia-public autoroll-google3

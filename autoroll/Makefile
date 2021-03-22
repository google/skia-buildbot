# This makefile is used to compile Autoroller code, build Docker images
# containing the same, and generate, apply, and push Kubernetes configs
# for the autorollers.

include ../kube/kube.mk

# First create a list of all the modules that ./go/rpc depends upon.
rpcDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' ./go/rpc)

# Then use that list to find all the Go files that make up those dependent modules.
rpcDependentFiles := ./go/rpc/rpc.proto $(shell echo $(rpcDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}')

rpcOutputFiles := ./go/rpc/rpc.pb.go ./go/rpc/rpc.twirp.go ./modules/rpc/rpc.ts ./modules/rpc/twirp.ts

$(rpcOutputFiles): $(rpcDependentFiles)
	go generate ./go/rpc

./package-lock.json: package.json
	npm install

./node_modules/lastupdate: ./package-lock.json
	npm ci
	touch ./node_modules/lastupdate

pages := $(shell find ./pages -type f)
modules := $(shell find ./modules -type f)
frontendInputFiles := $(pages) $(modules) $(rpcOutputFiles) ./node_modules/lastupdate
pageBaseNames := $(sort $(basename $(notdir $(pages))))
distNames := $(addsuffix -bundle.js,$(pageBaseNames)) $(addsuffix -bundle.css,$(pageBaseNames)) $(addsuffix .html,$(pageBaseNames))
dist := $(addprefix ./dist/,$(distNames))

$(dist): $(frontendInputFiles)
	npx webpack --mode=production

.PHONY: autoroll-fe
autoroll-fe: $(dist)
	CGO_ENABLED=0 GOOS=linux go install -v ./go/autoroll-fe

.PHONY: autoroll-be
autoroll-be:
	GOOS=linux go install -v ./go/autoroll-be

.PHONY: autoroll-google3
autoroll-google3:
	CGO_ENABLED=0 GOOS=linux go install -v ./go/autoroll-google3

.PHONY: all
all: autoroll-fe autoroll-be autoroll-google3

.PHONY: serve
serve: $(dist)
	npx webpack-dev-server --mode=development --watch-poll

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
release-be: autoroll-be build_base_cipd_release
	./build_backend_release

.PHONY: release-fe
release-fe: autoroll-fe
	./build_frontend_release

.PHONY: release-google3
release-google3: autoroll-google3 setup-internal-config-repo
	CONFIG_DIR="$(ROLLER_INTERNAL_CONFIG_DIR)" ./build_google3_release "$(MESSAGE)"

.PHONY: release-all
release-all: release-fe release-be release-google3

# autoroll-pusher is used by the below targets to apply k8s configurations.
.PHONY: autoroll-pusher
autoroll-pusher: kube-conf-gen deployment-dirs
	go install -v ./go/autoroll-pusher

# apply-roller generates a new backend release build and applies it, along with
# the current roller config file(s), to the roller(s) matching the ROLLER
# environment variable. Also updates the frontend with the current roller
# config(s) but does not change the frontend build.
.PHONY: apply-roller
apply-roller: release-be autoroll-pusher setup-internal-config-repo
	autoroll-pusher --roller="$(ROLLER)" --update-config --update-be-image

# push-roller is the same as apply-roller, but it also commits and pushes the
# modified kubernetes config files into the git repo, using the commit message
# from the MESSAGE environment variable.
.PHONY: push-roller
push-roller: release-be autoroll-pusher setup-internal-config-repo
	autoroll-pusher --roller="$(ROLLER)" --update-config --update-be-image --commit-with-msg="$(MESSAGE)" --use-tmp-checkout

# push-be generates a new backend release build and applies it to ALL rollers,
# then commits and pushes the modified kubernetes config files into the git
# repo, using the commit message from the MESSAGE environment variable. Does NOT
# update the roller config files used by the backends.
.PHONY: push-be
push-be: release-be autoroll-pusher setup-internal-config-repo
	autoroll-pusher --update-be-image --commit-with-msg="$(MESSAGE)" --use-tmp-checkout

# push-fe generates a new frontend release build, applies it, and commits and
# pushes the modified kubernetes config files into the git repo, using the
# commit message from the MESSAGE environment variable. Does NOT update the
# roller config files used by the frontends.
.PHONY: push-fe
push-fe: release-fe autoroll-pusher setup-internal-config-repo
	autoroll-pusher --update-fe-image --commit-with-msg="$(MESSAGE)" --use-tmp-checkout

# push-all generates new frontend and backend release builds, applies them along
# with the current roller config files(s), and commits and pushes the modified
# kubernetes config files into the git repo, using the commit message from the
# MESSAGE environment variable.
.PHONY: push-all
push-all: release-be release-fe autoroll-pusher setup-internal-config-repo
	autoroll-pusher --update-config --update-be-image --update-fe-image --commit-with-msg="$(MESSAGE)" --use-tmp-checkout

# validate runs the validate_autoroll_config task driver using the provided
# CONFIG variable, which may be a single config file or a directory containing
# config files.
.PHONY: validate
validate:
	go run ../infra/bots/task_drivers/validate_autoroll_configs/validate_autoroll_configs.go --local --config="$(CONFIG)"

# Run the autoroll-fe locally. This is just for convenience in testing.
.PHONY: run-fe
run-fe: autoroll-fe
	autoroll-fe --local --logtostderr --firestore_instance=staging --config_file=./config/skia-skiabot-test.json

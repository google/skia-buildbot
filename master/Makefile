# -*- makefile -*-

CWD := $(shell pwd)

THIRDPARTY := $(CWD)/../third_party
CHROMIUM_BUILDBOT := $(THIRDPARTY)/chromium_buildbot
PUBLICCONFIG_DIR := $(CHROMIUM_BUILDBOT)/site_config
PRIVATECONFIG_DIR ?= $(CWD)/../site_config

ALL_TARGETS := start stop reconfig log wait restart upgrade

.PHONY: $(ALL_TARGETS)
$(ALL_TARGETS)::
	$(MAKE) -C $(CHROMIUM_BUILDBOT)/masters/master.client.skia $@ PRIVATECONFIG_DIR=$(PRIVATECONFIG_DIR)

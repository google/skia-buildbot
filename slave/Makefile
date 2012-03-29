# -*- makefile -*-

CWD := $(shell pwd)

THIRDPARTY := $(CWD)/../third_party
CHROMIUM_BUILDBOT := $(THIRDPARTY)/chromium_buildbot
PUBLICCONFIG_DIR := $(CHROMIUM_BUILDBOT)/site_config
PRIVATECONFIG_DIR ?= $(CWD)/../site_config

ALL_TARGETS := start stop reconfig log wait restart

.PHONY: $(ALL_TARGETS)
$(ALL_TARGETS)::
	echo "fake bot_password to quiet run_slave.py" >$(PUBLICCONFIG_DIR)/.bot_password
	PYTHONPATH=$(PRIVATECONFIG_DIR) $(MAKE) -C $(CHROMIUM_BUILDBOT)/slave $@ PRIVATECONFIG_DIR=$(PRIVATECONFIG_DIR)

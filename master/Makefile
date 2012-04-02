# -*- makefile -*-

CWD := $(shell pwd)
THIRDPARTY := $(CWD)/../third_party
CHROMIUM_BUILDBOT := $(THIRDPARTY)/chromium_buildbot

# These variables are used within master-common.mk
MASTERPATH := master.client.skia
BUILDBOT_PATH = $(BUILDBOT8_PATH)
TOPLEVEL_DIR := $(CHROMIUM_BUILDBOT)
PUBLICCONFIG_DIR := $(CHROMIUM_BUILDBOT)/site_config
PRIVATECONFIG_DIR := $(CWD)/../site_config

include $(CHROMIUM_BUILDBOT)/masters/master-common.mk

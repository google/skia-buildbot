# -*- makefile -*-

THIRDPARTY := ../third_party
CHROMIUM_BUILDBOT := $(THIRDPARTY)/chromium_buildbot
PUBLIC_CONFIG := $(CHROMIUM_BUILDBOT)/site_config
PRIVATE_CONFIG := ../site_config

ALL_TARGETS := start stop reconfig log wait restart upgrade

# TODO(epoger): change master.client.skia/Makefile so that we can add to its
# PYTHONPATH rather than copying the file.
#
# TODO(epoger): figure out how to properly override Master.bot_password in the
# main config.py, rather than copying it from $(PRIVATE_CONFIG)/bot_password
#
.PHONY: $(ALL_TARGETS)
$(ALL_TARGETS)::
	cp $(PRIVATE_CONFIG)/config_private.py $(PUBLIC_CONFIG)
	cp $(PRIVATE_CONFIG)/bot_password $(PUBLIC_CONFIG)/.bot_password
	$(MAKE) -C $(CHROMIUM_BUILDBOT)/masters/master.client.skia $@

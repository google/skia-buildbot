# -*- makefile -*-

CHROMIUM_BUILDBOT := ../third_party/chromium_buildbot
ALL_TARGETS := start stop reconfig log wait restart upgrade

# TODO(epoger): change master.client.skia/Makefile so that we can add to its
# PYTHONPATH rather than copying the file.
#
# TODO(epoger): figure out how to properly override Master.bot_password in
# the main config.py, rather than writing the fake password into a file.
#
.PHONY: $(ALL_TARGETS)
$(ALL_TARGETS)::
	cp ../site_config/config_private.py $(CHROMIUM_BUILDBOT)/site_config
	echo epoger-temp-password >$(CHROMIUM_BUILDBOT)/site_config/.bot_password
	$(MAKE) -C $(CHROMIUM_BUILDBOT)/masters/master.client.skia $@

# Makefile to simplify uploads.
# Use 'make help' for a list of commands.

# A Makefile is used here to be consistent with some other AppEngine frameworks.
# For example Rietveld, which is where this file is derived from:
# https://code.google.com/p/rietveld/source/browse/Makefile

# Helper code to detect SDK location
define DETECT_SDK
import os
locations = [
  "../google_appengine",
  "/usr/local/google_appengine",
  ".locally/google_appengine",
]
for path in locations:
  if os.path.exists(path):
    print(path)
    break
endef
# /Helper

APPID?= `cat app.yaml | sed -n 's/^application: *//p'`

SDK_PATH ?= $(shell python -c '$(DETECT_SDK)')

APPCFG?= $(if $(SDK_PATH), $(SDK_PATH)/,)appcfg.py
APPCFG_FLAGS?=

# Set dirty suffix depending on if there are changes not yet pushed to the repo.
dirty=
ifneq ($(shell git status -s),)
        dirty="-tainted"
endif
VERSION= `git rev-parse HEAD`$(dirty)

PYTHON?= python2.7
COVERAGE?= coverage


default: help

help:
	@echo "Available commands:"
	@sed -n '/^[a-zA-Z0-9_.]*:/s/:.*//p' <Makefile | sort

update:
	@echo "---[Updating $(APPID)]---"
	$(APPCFG) $(APPCFG_FLAGS) update . --application $(APPID) --version $(VERSION)

upload: update

deploy: update

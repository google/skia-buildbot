THIS_FILE := $(abspath $(lastword $(MAKEFILE_LIST)))
WEBPACK_DIR := $(dir $(THIS_FILE))

.PHONY: webpack
webpack:
	cd $(WEBPACK_DIR) && npx webpack --mode=development

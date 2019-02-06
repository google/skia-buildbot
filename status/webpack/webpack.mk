THIS_FILE := $(abspath $(lastword $(MAKEFILE_LIST)))
WEBPACK_DIR := $(dir $(THIS_FILE))

$(WEBPACK_DIR)/package-lock.json: $(WEBPACK_DIR)/package.json
	cd $(WEBPACK_DIR) && npm install

.PHONY: webpack
webpack: $(WEBPACK_DIR)/package-lock.json
	cd $(WEBPACK_DIR) && npx webpack --mode=development

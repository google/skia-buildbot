DEPOT_TOOLS_MK_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

depot_tools:
	echo $(DEPOT_TOOLS_MK_DIR)
	python $(DEPOT_TOOLS_MK_DIR)/generate.py $(DEPOT_TOOLS_MK_DIR)/gen_version.inp $(DEPOT_TOOLS_MK_DIR)/gen_version.go

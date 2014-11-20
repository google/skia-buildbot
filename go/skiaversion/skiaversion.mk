DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

skiaversion:
	python $(DIR)/generate.py $(DIR)/gen_version.inp $(DIR)/gen_version.go


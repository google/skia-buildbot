COMMIT=`git rev-parse HEAD`
DATE=`date --rfc-3339="seconds"`
DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

skiaversion:
	echo -e "package skiaversion\n\nconst(\n\tCOMMIT = \"$(COMMIT)\"\n\tDATE = \"$(DATE)\"\n)\n" > $(DIR)/gen_version.go

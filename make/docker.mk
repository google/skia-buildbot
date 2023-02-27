# Helper rules for Docker.

# DOCKER defines which executable to run.
#
ifeq ($(DOCKER),)
	ifneq ($(strip $(shell which docker)),)
		DOCKER := $(strip $(shell which docker))
	else
		DOCKER := docker
	endif
endif

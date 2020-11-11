include ../../../res/imp/http-server.mk

run: install_extras

install_extras:
	# We download these here because the default target is called by unittests
	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/task-scheduler-demo/job-search-sk-demo.json

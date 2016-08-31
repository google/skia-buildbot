# Copyright 2014 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
projdir := $(patsubst %/,%,$(dir $(mkfile_path)))

dev:
	$(projdir)/../../../../../../luci/appengine/components/tools/gae.py devserver  -- --host 0.0.0.0 --port 8082 --admin_port 7999 --log_level debug

up:
	$(projdir)/../../../../../../luci/appengine/components/tools/gae.py upload -A luci-milo --app-dir $(projdir)/appengine

test:
	go test ./...

train:
	go test ./... -test.generate

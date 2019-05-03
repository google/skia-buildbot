.PHONY: default
default: run

# Find the name of the server based on our CWD.
# Assume we're in a subdirectory of $GOPATH/src/go.skia.org/infra, trim
# everything up to and including "infra", plus any "/res/imp" suffix. For most
# servers that gets us the server name. Some others also have a "frontend"
# subdir, which we also trim. For the common "res/imp" directory, we just use
# that.
server = $(shell pwd | sed 's/.*infra\///' | sed 's/\/\(frontend\/\)\?res\/imp//')

# Derive a 4-digit port number starting with '8' by hashing the server name.
port = 8$(shell printf "%03d" $(shell expr $(shell cksum <<< "$(server)" | cut -f 1 -d ' ') % 999))

# Determine where http-server should be.
bower_dir = $(shell realpath $(shell if [ -f bower.json ]; then echo "."; else echo "../.."; fi))
common_dir = $(shell realpath $(shell if [ -f bower.json ]; then echo ".."; else echo "../../../res"; fi))
httpserver = python -m SimpleHTTPServer

# List of all dependencies.
deps_list = bower_components res/imp/bower_components res/common res/img res/js res/imp/sinon-1.17.2.js

bower_components: $(bower_dir)/third_party/bower_components
	ln -sfT $(bower_dir)/third_party/bower_components bower_components

$(bower_dir)/third_party/bower_components:
	cd $(bower_dir) && bower install

res/imp/bower_components: $(bower_dir)/third_party/bower_components
	mkdir -p res/imp
	ln -sfT $(bower_dir)/third_party/bower_components res/imp/bower_components

res/common:
	mkdir -p res/imp
	ln -sfT $(common_dir) res/common

res/img:
	mkdir -p res/imp
	ln -sfT $(bower_dir)/res/img res/img

res/js:
	mkdir -p res/imp
	ln -sfT $(bower_dir)/res/js res/js

res/imp/sinon-1.17.2.js:
	mkdir -p res/imp
	npm install sinon@1.17.2
	cp $(bower_dir)/node_modules/sinon/pkg/sinon-1.17.2.js res/imp/sinon-1.17.2.js

# Top-level targets.

# Used for setting things up without running.
.PHONY: deps
deps: $(deps_list)

# Run a local HTTP server for the demo pages.
.PHONY: run
run: $(deps_list)
	$(httpserver) $(port)

# Print out some critical information for debugging.
.PHONY: echo
echo:
	echo $(bower_dir)
	echo $(common_dir)
	echo $(server)
	echo $(port)
	echo $(httpserver)

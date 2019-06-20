.PHONY: default
default: run

httpserver = python -m SimpleHTTPServer

# List of all dependencies.
deps_list = res/common

res/common:
	mkdir res
	ln -sfT ../.. res/common

# Top-level targets.

# Used for setting things up without running.
.PHONY: deps
deps: $(deps_list)

# Run a local HTTP server for the demo pages.
.PHONY: run
run: $(deps_list)
	npm ci
	$(httpserver) $(port)

# Print out some critical information for debugging.
.PHONY: echo
echo:
	echo $(server)
	echo $(port)
	echo $(httpserver)

.PHONY: default
default: run

# Determine where http-server should be.
httpserver = python -m SimpleHTTPServer

# Top-level targets.

# Used for setting things up without running.
.PHONY: deps
deps:
	npm i

# Run a local HTTP server for the demo pages.
.PHONY: run
run:
	$(httpserver) $(port)

# Print out some critical information for debugging.
.PHONY: echo
echo:
	echo $(server)
	echo $(port)
	echo $(httpserver)

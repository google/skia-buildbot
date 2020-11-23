"""This module defines the sk_demo_page_server macro."""

_demo_page_server = "//infra-sk/sk_demo_page_server:demo_page_server"

_demo_server_script_template = """
#!/bin/bash
#
# Runs the demo page server.

HTML_FILE=$(rootpath {html_file})
DEMO_PAGE_SERVER_BIN=$(rootpath {demo_page_server})
PORT={port}

# We won't serve the given HTML file directly. Instead, we'll serve its parent directory, which we
# assume contains the JS/CSS bundles and any other required assets.
ASSETS_DIR=$$(dirname $$HTML_FILE)

# Copy the HTML file as index.html so it is served by default at http://localhost:<port>/.
cp -f $$HTML_FILE $$ASSETS_DIR/index.html

# Start the demo page server.
$$DEMO_PAGE_SERVER_BIN --directory $$ASSETS_DIR --port $$PORT
exit $$?
"""

def sk_demo_page_server(name, sk_page, port = 8080):
    """Creates a demo page server for the given page.

    This target can be used during development with "bazel run".

    It can also be used as the environment for a test_on_env test (e.g. for Puppeteer tests). The
    TCP port for the HTTP server can be found in $ENV_DIR/port.

    Args:
      name: Name of the rule.
      sk_page: Label of the sk_page to serve.
      port: Port for the HTTP server. Set to 0 to let the OS choose an unused port.
    """
    script = _demo_server_script_template.format(
        html_file = sk_page + "_html_dev",
        demo_page_server = _demo_page_server,
        port = port,
    )

    native.genrule(
        name = name + "_genrule",
        srcs = [
            # Label for the development .html file produced by the sk_page rule. We need to list
            # this explicitly in the srcs attribute in order for the $(rootpath {html_file})
            # variable expansion to work.
            sk_page + "_html_dev",
            _demo_page_server,
        ],
        outs = [name + ".sh"],
        cmd = "echo '{}' > $@".format(script.replace("'", "'\"'\"'")),
        executable = 1,
    )

    native.sh_binary(
        name = name,
        srcs = [name + "_genrule"],
        data = [
            _demo_page_server,
            # This label includes the development .html file produced by the sk_page rule and all
            # its assets (JS/CSS development bundles, etc.).
            sk_page + "_dev",
        ],
    )

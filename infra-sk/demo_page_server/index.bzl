_demo_page_server = "//infra-sk/demo_page_server:demo_page_server"

_demo_server_script_template = """
#!/bin/bash
#
# Runs the demo page server.

echo PWD: $$PWD

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

def demo_page_server(name, sk_page, port=8080):
    script = _demo_server_script_template.format(
        html_file = sk_page + "_html_dev",
        demo_page_server = _demo_page_server,
        port = port,
    )

    native.genrule(
        name = name + "_genrule",
        srcs = [
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
            # Demo page and assets (HTML file, plus JS and CSS development bundles).
            "%s_dev" % sk_page,
        ],
    )

    # native.sh_binary(
    #     name = name,
    #     srcs = ["//infra-sk/demo_page_server:demo_page_server.sh"],
    #     data = [
    #         "//infra-sk/demo_page_server:demo_page_server",
    #         # Demo page and assets (HTML file, plus JS and CSS development bundles).
    #         "%s_dev" % sk_page,
    #         # Demo page HTML file alone. This is redundant with the above target, but we need to
    #         # list it here explicitly in order to reference it from the args attribute below.
    #         "%s_html_dev" % sk_page,
    #     ],
    #     args = [
    #         "--demo-page-server-bin $(location //infra-sk/demo_page_server:demo_page_server)",
    #         "--html-file $(location %s_html_dev)" % sk_page,
    #         "--port %d" % port,
    #     ]
    # )
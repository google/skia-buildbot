def demo_page_server(name, sk_page, port=8080):
    native.sh_binary(
        name = name,
        srcs = ["//infra-sk/demo_page_server:demo_page_server.sh"],
        data = [
            "//infra-sk/demo_page_server:demo_page_server",
            # Demo page and assets (HTML file, plus JS and CSS development bundles).
            "%s_dev" % sk_page,
            # Demo page HTML file alone. This is redundant with the above target, but we need to
            # list it here explicitly in order to reference it from the args attribute below.
            "%s_html_dev" % sk_page,
        ],
        args = [
            "--demo-page-server-bin $(location //infra-sk/demo_page_server:demo_page_server)",
            "--html-file $(location %s_html_dev)" % sk_page,
            "--port %d" % port,
        ]
    )


"""
Creates a puppeteer config file using Make variable substitution.
"""

def _puppeteerrc_js(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".js")
    ctx.actions.expand_template(
        output = out,
        substitutions = {
            "{BROWSER_EXE}": ctx.var["CHROMEDRIVER"],
            #            "{CHROMEDRIVER}": ctx.var["CHROMEDRIVER"],  or CHROME-HEADLESS-SHELL
        },
        template = ctx.file.template,
    )
    return [DefaultInfo(files = depset([out]))]

puppeteerrc_js = rule(
    doc = """
We want puppeteer to use a hermetic browser in puppeteer.
We download the browser via @rules_browsers, which
can be loaded as a toochain that sets the CHROME and
CHROMEDRIVER Make variables. This rule uses those
Make variables to do template expansion on a
puppeteerrc.js template file.

Note that when using this rule the toolchain must be supplied,
which permits using different browsers for puppeteer tests.

For example:

        puppeteerrc_js(
            name = "puppeteerrc_js_expanded",
            template = "puppeteerrc.js.tpl",
            toolchains = ["@rules_browsers//browsers/chromium:toolchain_alias"],
        )



    """,
    attrs = {
        "template": attr.label(
            allow_single_file = [".js.tpl"],
            mandatory = True,
        ),
    },
    implementation = _puppeteerrc_js,
)

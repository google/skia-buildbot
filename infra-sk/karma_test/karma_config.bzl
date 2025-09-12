"""
Creates a karma config file using Make variable substitution.
"""

def _karma_config_js(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".js")
    ctx.actions.expand_template(
        output = out,
        substitutions = {
            "{BROWSER_EXE}": ctx.var["CHROME-HEADLESS-SHELL"],
        },
        template = ctx.file.template,
    )
    return [DefaultInfo(files = depset([out]))]

karma_config_js = rule(
    doc = """
We want karma_test to use a hermetic browser. We download the 
browser via @rules_browsers, which can be loaded as a toochain 
that sets the CHROME-HEADLESS-SHELL Make variable. This rule 
uses those Make variables to do template expansion on a *.js.tpl
template file.

Note that when using this rule the toolchain must be supplied,
which permits using different browsers for karma tests.

For example:

        karma_config_js(
            name = "karma_config_js_expanded",
            template = "karma_config.js.tpl",
            toolchains = ["@rules_browsers//browsers/chromium:toolchain_alias"],
        )

    """,
    attrs = {
        "template": attr.label(
            allow_single_file = [".js.tpl"],
            mandatory = True,
        ),
    },
    implementation = _karma_config_js,
)

"""This module provides a wrapper around the ts_project rule from the rules_ts repository."""

load("@aspect_rules_ts//ts:defs.bzl", "ts_project")

def ts_library(name, srcs, deps = [], **kwargs):
    """Wraps rules_ts's ts_project rule with common settings for our code.

    This macro ensures all ts_project[1] rules use our common //tsconfig.json file, and prevents
    errors such as "error TS2688: Cannot find type definition file for 'mocha'", which arise when
    the tsconfig.json file declares ambient modules[2] via compilerOptions.types[3], but we forget
    to provide them to the ts_project rule via the deps argument. This macro simply adds those
    dependencies for us.

    This macro is called ts_library, as opposed to ts_project, for consistency with our other
    *_library rules, and also because it used to be a wrapper around the now-deprecated ts_library
    rule from the rules_nodejs repository[4].

    [1] https://docs.aspect.build/rulesets/aspect_rules_ts/docs/rules#ts_project
    [2] https://www.typescriptlang.org/docs/handbook/modules.html#ambient-modules
    [3] https://www.typescriptlang.org/tsconfig#types
    [4] https://bazelbuild.github.io/rules_nodejs/TypeScript.html#option-4-ts_library

    Args:
      name: The name of the target.
      srcs: A list of TypeScript source files.
      deps: A list of TypeScript dependencies.
      **kwargs: Any other arguments to pass to the ts_library rule.
    """

    # Keep in sync with the "types" field in //tsconfig.json.
    ambient_types = [
        "//:node_modules/@types/mocha",
        "//:node_modules/@types/node",
    ]

    ts_project(
        name = name,
        srcs = srcs,
        deps = deps + [dep for dep in ambient_types if dep not in deps],
        tsconfig = "//:ts_config",
        declaration = True,
        transpiler = "tsc",
        allow_js = True,
        **kwargs
    )

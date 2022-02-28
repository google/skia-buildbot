"""This module provides a wrapper around the ts_project rule from the rules_nodejs repository."""

load("@npm//@bazel/typescript:index.bzl", "ts_project")

def ts_library(name, srcs, deps = [], **kwargs):
    """Wraps rules_nodejs's ts_project rule to include ambient types declared in //tsconfig.json.

    This macro prevents errors such as "error TS2688: Cannot find type definition file for 'mocha'",
    which arise when the //tsconfig.json file declares ambient modules[2] via
    compilerOptions.types[3], but we forget to provide them to the ts_project rule via the deps
    argument. This macro simply adds those dependencies for us.

    ```
        // tsconfig.json
        {
          "compilerOptions": {
            ...
            "types": ["mocha", "node"],  // Ambient type declarations.
          }
        }

        // BUILD.bazel
        ts_project(
          name = "example",
          srcs = ["example.ts"],
          deps = [
            "@types/mocha",  # Added by this macro.
            "@types/node",   # Added by this macro.
            ...
          ],
        )
    ```

    This macro is called ts_library, as opposed to ts_project, for consistency with our other
    *_library rules, and also because it used to be a wrapper around the now-deprecated ts_library
    rule from the rules_nodejs repository[4].

    [1] https://bazelbuild.github.io/rules_nodejs/TypeScript.html#ts_project
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
        "@npm//@types/mocha",
        "@npm//@types/node",
    ]

    ts_project(
        name = name,
        srcs = srcs,
        deps = deps + [dep for dep in ambient_types if dep not in deps],
        # These options emulate the behavior of the now deprecated ts_library rule from the
        # rules_nodejs repository. Our tsconfig.json produces JavaScript files compatible with Node
        # by default, which is great for e.g. Puppeteer tests because they don't make use of this
        # rule. For browser bundles, we override these settings such that the .js and .d.ts files
        # produced by the TypeScript compiler can be consumed by rollup_bundle.
        tsconfig = {
            "compilerOptions": {
                "module": "esnext",
                "moduleResolution": "node",
            },
        },
        extends = "//:tsconfig.json",
        declaration = True,
        allow_js = True,
        **kwargs
    )

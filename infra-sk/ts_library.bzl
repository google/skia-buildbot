"""This module provides a wrapper around the ts_library rule from the rules_nodejs repository."""

load("@infra-sk_npm//@bazel/typescript:index.bzl", _ts_library = "ts_library")

def ts_library(name, srcs, deps = [], **kwargs):
    """Wraps rules_nodejs's ts_library rule to include ambient types declared in //tsconfig.json.

    The ts_library[1] rule from the rules_nodejs repository ignores[2] any ambient types[3] declared
    via the `{"compilerOptions": {"types": [...]}}` field[4] in //tsconfig.json. This causes
    TypeScript files using said types to fail to compile (e.g. "TS2304: Cannot find name 'XXX'").
    Code editors, however, would not report any such errors because they correctly interpret the
    "types" field in //tsconfig.json.

    The solution is to explicitly include the ambient types in the deps argument of the ts_library
    rule, e.g.:

    ```
        // tsconfig.json
        {
          "compilerOptions": {
            ...
            "types": ["mocha", "node"],  // Ambient type declarations.
          }
        }

        // BUILD.bazel
        ts_library(
          name = "example",
          srcs = ["example.ts"],
          deps = [
            "@types/mocha",  # Ambient type declared in //tsconfig.json.
            "@types/node",   # Ambient type declared in //tsconfig.json.
            ...              # Any other deps.
          ],
        )
    ```

    This wrapper around the ts_library rule eliminates said compilation errors by automatically
    including as dependencies any ambient types declared in the //tsconfig.json file.

    [1] https://bazelbuild.github.io/rules_nodejs/TypeScript.html#ts_library
    [2] https://github.com/bazelbuild/rules_nodejs/blob/92e816986e19b9a68091a667f206d8589393eb74/packages/typescript/internal/build_defs.bzl#L248
    [3] https://www.typescriptlang.org/docs/handbook/modules.html#ambient-modules
    [4] https://www.typescriptlang.org/tsconfig#types

    Args:
      name: The name of the target.
      srcs: A list of TypeScript source files.
      deps: A list of TypeScript dependencies.
      **kwargs: Any other arguments to pass to the ts_library rule.
    """

    # Keep in sync with the "types" field in //tsconfig.json.
    ambient_types = [
        "@infra-sk_npm//@types/mocha",
        "@infra-sk_npm//@types/node",
    ]

    _ts_library(
        name = name,
        srcs = srcs,
        deps = deps + [dep for dep in ambient_types if dep not in deps],
        **kwargs
    )

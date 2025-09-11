load("@aspect_rules_js//npm/private:npm_translate_lock.bzl", "npm_translate_lock_rule")
# -- load statements -- #

def _extension_for_npm_translate_lock_rule_impl(ctx):
    npm_translate_lock_rule(
        name = "npm",
        additional_file_contents = {},
        bins = {},
        custom_postinstalls = {},
        data = [
            "//:package.json",
        ],
        dev = False,
        external_repository_action_cache = ".aspect/rules/external_repository_action_cache",
        lifecycle_hooks_envs = {},
        lifecycle_hooks_execution_requirements = {
            "*": [
                "no-sandbox",
            ],
        },
        lifecycle_hooks = {
            "*": [
                "preinstall",
                "install",
                "postinstall",
            ],
        },
        no_optional = False,
        npm_package_lock = "//:package-lock.json",
        npmrc = "//:.npmrc",
        package_visibility = {},
        patch_args = {
            "*": [
                "-p0",
            ],
        },
        patches = {},
        pnpm_lock = "//:pnpm-lock.yaml",
        preupdate = [],
        prod = False,
        public_hoist_packages = {},
        quiet = True,
        update_pnpm_lock = True,
        update_pnpm_lock_node_toolchain_prefix = "nodejs",
        verify_node_modules_ignored = "//:.bazelignore",
        npm_package_target_name = "{dirname}",
    )

# -- repo definitions -- #

extension_for_npm_translate_lock_rule = module_extension(implementation = _extension_for_npm_translate_lock_rule_impl)

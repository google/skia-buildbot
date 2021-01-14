load("@bazel_tools//tools/build_defs/cc:action_names.bzl", "ACTION_NAMES")
load(
    "@bazel_tools//tools/cpp:cc_toolchain_config_lib.bzl",
    "feature",
    "flag_group",
    "flag_set",
    "tool_path",
    "action_config",
    "tool",
)

all_link_actions = [
    ACTION_NAMES.cpp_link_executable,
    ACTION_NAMES.cpp_link_dynamic_library,
    ACTION_NAMES.cpp_link_nodeps_dynamic_library,
]

def _impl(ctx):
    print('running the implementation')
    features = [
        feature(
            name = "default_linker_flags",
            enabled = True,
            flag_sets = [
                flag_set(
                    actions = all_link_actions,
                    flag_groups = ([
                        flag_group(
                            flags = [
                                "-lstdc++",
                            ],
                        ),
                    ]),
                ),
            ],
        ),
    ]

    return cc_common.create_cc_toolchain_config_info(
        ctx = ctx,
        features = features,
        # we may need to use gcc -no-canonical-prefixes and -fno-canonical-system-headers?
        cxx_builtin_include_directories = [
            "/usr/lib/llvm-11/lib/clang/11.0.0/include",
            "/usr/include",
   #         "/usr/lib/gcc/x86_64-linux-gnu/10/include",
        ],
        toolchain_identifier = "local",
        host_system_name = "local",
        target_system_name = "local",
        target_cpu = "some_cpu",
        target_libc = "unknown",
        compiler = "unknown",
        abi_version = "unknown",
        abi_libc_version = "unknown",
        action_configs = [action_config(
            action_name=ACTION_NAMES.cpp_compile,
            enabled=True,
            tools=[
                tool(
                    path= ctx.attr.compiler_path,
                )
            ]
        ),
        action_config(
            action_name=ACTION_NAMES.cpp_link_executable,
            enabled=True,
            tools=[
                tool(
                    path= ctx.attr.compiler_path,
                )
            ]
        )],
    )#,
    #DefaultInfo(
    #    executable = out
    #),

cc_toolchain_config = rule(
    implementation = _impl,
    attrs = {
        "compiler_path": attr.string(mandatory=True),
        "deps": attr.label_list(cfg="exec"),
    },
    provides = [CcToolchainConfigInfo],
    #executable = True,
)
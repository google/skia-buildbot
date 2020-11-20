
def test_on_env(name, test, env, timeout_secs=10):
    native.sh_test(
        name = name,
        srcs = ["//bazel:test_on_env.sh"],
        args = [
            "--test-bin $(location %s)" % test,
            "--env-bin $(location %s)" % env,
            "--ready-check-timeout-secs %d" % timeout_secs,
        ],
        data = [test, env],
    )

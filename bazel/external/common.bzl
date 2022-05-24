"""This module defines utilities shared between the repository rules in this directory."""

def fail_if_nonzero_status(exec_result, msg):
    """Fails if the result of the given exec_result contains a non-zero return code.

    This function takes an exec_result[1] structure returned by the repository_ctx.execute[2]
    method, and fails if the return code is non-zero. It prints out the given msg and the failed
    command's exit code, stdout and stderr.

    [1] https://bazel.build/rules/lib/exec_result
    [2] https://bazel.build/rules/lib/repository_ctx#execute

    Args:
        exec_result: An exec_result.
        msg: A message describing the command that failed.
    """

    if exec_result.return_code != 0:
        fail("%s\nExit code: %d\nStdout:\n%s\nStderr:\n%s\n" % (
            msg,
            exec_result.return_code,
            exec_result.stdout,
            exec_result.stderr,
        ))

#!/usr/bin/env python3
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Presubmit checks for the Skia infrastructure code."""

USE_PYTHON3 = True

def _RunPresubmitsWithBazelisk(input_api, output_api, extra_arg):
  """Run presubmit.go via bazelisk. Fail if it returns a non-zero exit code."""
  command = ['bazelisk', 'run', '//cmd/presubmit', '--', extra_arg]
  command_str = ' '.join(command)
  results = []

  print('Running "%s" ...' % command_str)
  try:
    input_api.subprocess.check_output(
        command,
        stderr=input_api.subprocess.STDOUT,
        encoding='utf-8')
  except input_api.subprocess.CalledProcessError as e:
    results += [output_api.PresubmitPromptWarning(
        'Command "%s" returned non-zero exit code %d. Output: \n\n%s' % (
            command_str,
            e.returncode,
            e.output,
        )
    )]

  return results


def CheckChangeOnUpload(input_api, output_api):
  return _RunPresubmitsWithBazelisk(input_api, output_api, '--upload')


def CheckChangeOnCommit(input_api, output_api):
  return _RunPresubmitsWithBazelisk(input_api, output_api, '--commit')

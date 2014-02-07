#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command and report its results in machine-readable format."""


import json
import subprocess
import sys


# We print this string before and after the important output from the command.
# This makes it easy to ignore output from SSH, shells, etc.
BOOKEND_STR = '@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@'


def encode_results(results):
  """Convert a dictionary of results into a machine-readable string.

  Args:
      results: dict, the results of a command.
  Returns:
      A JSONified string, bookended by BOOKEND_STR for easy parsing.
  """
  return (BOOKEND_STR + json.dumps(results) +
          BOOKEND_STR).decode('string-escape')


def decode_results(results_str):
  """Convert a machine-readable string into a dictionary of results.

  Args:
      results_str: string; output from local_run or one of its siblings.
  Returns:
      A dictionary of results.
  """
  return json.loads(results_str.split(BOOKEND_STR)[1])


def cmd_results(stdout='', stderr='', returncode=1):
  """Create a results dict for a command.

  Args:
      stdout: string; stdout from a command.
      stderr: string; stderr from a command.
      returncode: string; return code of a command.
  """
  return {'stdout': stdout.encode('string-escape'),
          'stderr': stderr.encode('string-escape'),
          'returncode': returncode}


def run(cmd):
  """Run the command, block until it completes, and return a results dictionary.

  Args:
      cmd: string or list of strings; the command to run.
  Returns:
      A dictionary with stdout, stderr, and returncode as keys.
  """
  try:
    proc = subprocess.Popen(cmd, shell=False, stderr=subprocess.PIPE,
                            stdout=subprocess.PIPE)
  except OSError as e:
    return cmd_results(stderr=str(e))
  stdout, stderr = proc.communicate()
  return cmd_results(stdout=stdout,
                     stderr=stderr,
                     returncode=proc.returncode)


if '__main__' == __name__:
  print encode_results(run(sys.argv[1:]))

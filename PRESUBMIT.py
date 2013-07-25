# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Presubmit checks for the buildbot code."""

import os
import subprocess
import sys


SKIA_TREE_STATUS_URL = 'http://skia-tree-status.appspot.com'
SKIP_RUNS_KEYWORD = '(SkipBuildbotRuns)'


def _RunBuildbotTests(input_api, output_api):
  """ Run the buildbot tests and return a list of strings containing any errors.
  """
  results = []
  success = True
  try:
    proc = subprocess.Popen(['python', 'run_unittests'],
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    success = proc.wait() == 0
    long_text = proc.communicate()[0]
  except Exception:
    success = False
    long_text = 'Failed to run the buildbot tests!'
  if not success:
    results.append(output_api.PresubmitPromptWarning(
        message='One or more buildbot tests failed.',
        long_text=long_text))
  return results


def CheckChange(input_api, output_api):
  """Presubmit checks for the change on upload or commit.

  The presubmit checks have been handpicked from the list of canned checks
  here:
  http://src.chromium.org/viewvc/chrome/trunk/tools/depot_tools/presubmit_canned_checks.py

  The following are the presubmit checks:
  * Pylint is run if the change contains any .py files.
  * Enforces max length for all lines is 80.
  * Checks that the user didn't add TODO(name) without an owner.
  * Checks that there is no stray whitespace at source lines end.
  * Checks that there are no tab characters in any of the text files.
  """
  results = []

  pylint_disabled_warnings = (
      'F0401',  # Unable to import.
      'E0611',  # No name in module.
      'W0232',  # Class has no __init__ method.
      'E1002',  # Use of super on an old style class.
      'W0403',  # Relative import used.
      'R0201',  # Method could be a function.
      'E1003',  # Using class name in super.
      'W0613',  # Unused argument.
  )
  # Run Pylint on only the modified python files. Unfortunately it still runs
  # Pylint on the whole file instead of just the modified lines.
  affected_python_files = []
  for affected_file in input_api.AffectedSourceFiles(None):
    affected_file_path = affected_file.LocalPath()
    if affected_file_path.endswith('.py'):
      affected_python_files.append(affected_file_path)
  results += input_api.canned_checks.RunPylint(
      input_api, output_api,
      disabled_warnings=pylint_disabled_warnings,
      white_list=affected_python_files)

  # Use 100 for max length for files other than python. Python length is
  # already checked during the Pylint above.
  results += input_api.canned_checks.CheckLongLines(input_api, output_api, 100)
  results += input_api.canned_checks.CheckChangeTodoHasOwner(
      input_api, output_api)
  results += input_api.canned_checks.CheckChangeHasNoStrayWhitespace(
      input_api, output_api)
  results += input_api.canned_checks.CheckChangeHasNoTabs(input_api, output_api)

  results += _RunBuildbotTests(input_api, output_api)

  return results


def _CheckTreeStatus(input_api, output_api, json_url):
  """Check whether to allow commit.

  Args:
    input_api: input related apis.
    output_api: output related apis.
    json_url: url to download json style status.
  """
  results = []
  tree_status_results = input_api.canned_checks.CheckTreeIsOpen(
      input_api, output_api, json_url=json_url)
  display_skip_keyword_prompt = False
  tree_status = None
  if not tree_status_results:
    # Check for caution state only if tree is not closed.
    connection = input_api.urllib2.urlopen(json_url)
    status = input_api.json.loads(connection.read())
    connection.close()
    tree_status = status['message']
    if 'caution' in tree_status.lower():
      # Display a prompt only if we are in an interactive shell. Without this
      # check the commit queue behaves incorrectly because it considers
      # prompts to be failures.
      display_skip_keyword_prompt = True
  else:
    # pylint: disable=W0212
    tree_status = tree_status_results[0]._message
    display_skip_keyword_prompt = True

  if (display_skip_keyword_prompt
      and os.isatty(sys.stdout.fileno())
      and not SKIP_RUNS_KEYWORD in input_api.change.DescriptionText()):
    long_text = (
        '%s\nAre you sure the change should be submitted. If it should be '
        'submitted but not run on the buildbots you can use the %s keyword.' % (
            tree_status, SKIP_RUNS_KEYWORD))
    results.append(output_api.PresubmitPromptWarning(
        message=tree_status, long_text=long_text))
  return results


def CheckChangeOnUpload(input_api, output_api):
  return CheckChange(input_api, output_api)


def CheckChangeOnCommit(input_api, output_api):
  results = CheckChange(input_api, output_api)
  results.extend(
      _CheckTreeStatus(input_api, output_api, json_url=(
          SKIA_TREE_STATUS_URL + '/banner-status?format=json')))
  return results


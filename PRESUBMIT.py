# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Presubmit checks for the buildbot code."""


import subprocess


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


def _CheckNonAscii(input_api, output_api):
  """Check for non-ASCII characters and throw warnings if any are found."""
  results = []
  files_with_unicode_lines = []
  # We keep track of the longest file (in line count) so that we can pad the
  # numbers when displaying output. This makes it easier to see the indention.
  max_lines_in_any_file = 0
  file_extensions = ('bat', 'cfg', 'cmd', 'conf', 'css', 'gyp', 'gypi', 'htm',
                     'html', 'js', 'json', 'ps1', 'py', 'sh', 'tac', 'yaml')
  for affected_file in input_api.AffectedSourceFiles(None):
    affected_filepath = affected_file.LocalPath()
    if affected_filepath.split('.')[-1] not in file_extensions:
      continue
    unicode_lines = []
    with open(affected_filepath, 'r+b') as f:
      total_lines = 0
      for line in f:
        total_lines += 1
        try:
          line.decode('ascii')
        except UnicodeDecodeError:
          unicode_lines.append((total_lines, line.rstrip()))
    if unicode_lines:
      files_with_unicode_lines.append((affected_filepath, unicode_lines))
      if total_lines > max_lines_in_any_file:
        max_lines_in_any_file = total_lines

  if files_with_unicode_lines:
    padding = len(str(max_lines_in_any_file))
    long_text = 'The following files contain non-ASCII characters:\n'
    for filename, unicode_lines in files_with_unicode_lines:
      long_text += '  %s\n' % filename
      for line_num, line in unicode_lines:
        long_text += '    %s: %s\n' % (str(line_num).rjust(padding), line)
      long_text += '\n'
    results.append(output_api.PresubmitPromptWarning(
        message='Some files contain non-ASCII characters.',
        long_text=long_text))

  return results


def CheckChange(input_api, output_api):
  """Presubmit checks for the change on upload or commit.

  The presubmit checks have been handpicked from the list of canned checks
  here:
  https://chromium.googlesource.com/chromium/tools/depot_tools/+/master/presubmit_canned_checks.py

  The following are the presubmit checks:
  * Pylint is run if the change contains any .py files.
  * Enforces max length for all lines is 100.
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
  # already checked during the Pylint above. No max length for Go files.
  whitelist = ['go', 'html', 'py']
  results += input_api.canned_checks.CheckLongLines(input_api, output_api, 100,
      source_file_filter=lambda x: x.LocalPath().split('.')[-1] not in whitelist
      )
  results += input_api.canned_checks.CheckChangeTodoHasOwner(
      input_api, output_api)
  results += input_api.canned_checks.CheckChangeHasNoStrayWhitespace(
      input_api, output_api)
  results += input_api.canned_checks.CheckChangeHasNoTabs(input_api, output_api)

  results += _RunBuildbotTests(input_api, output_api)

  return results


def CheckChangeOnUpload(input_api, output_api):
  results = CheckChange(input_api, output_api)
  # Give warnings for non-ASCII characters on upload but not commit, since they
  # may be intentional.
  results.extend(_CheckNonAscii(input_api, output_api))
  return results


def CheckChangeOnCommit(input_api, output_api):
  results = CheckChange(input_api, output_api)
  return results

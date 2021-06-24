# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Presubmit checks for the buildbot code."""


import subprocess


def _MakeFileFilter(input_api,
                    include_extensions=None,
                    include_filenames=None,
                    exclude_extensions=None,
                    exclude_filenames=None):
  """Return a filter to pass to AffectedSourceFiles.

  The filter will include all files with a file extension in include_extensions,
  and will ignore all files with a file extension in exclude_extensions.

  If include_extensions is empty, all files, even those without any extension,
  are included.
  """
  include = [input_api.re.compile(r'.+')]
  if include_extensions:
    include = [input_api.re.compile(r'.+\.%s$' % ext)
               for ext in include_extensions]
  if include_filenames:
    include += [input_api.re.compile(r'.*%s$' % filename.replace('.', '\.'))
                for filename in include_filenames]

  exclude = []
  if exclude_extensions:
    exclude = [input_api.re.compile(r'.+\.%s$' % ext)
               for ext in exclude_extensions]
  if exclude_filenames:
    exclude += [input_api.re.compile(r'.*%s$' % filename.replace('.', '\.'))
                   for filename in exclude_filenames]
  if len(exclude) == 0:
    # If exclude is empty, the InputApi default is used, so always include at
    # least one regexp.
    exclude = [input_api.re.compile(r'^$')]

  return lambda x: input_api.FilterSourceFile(x, files_to_check=include,
                                              files_to_skip=exclude)

def _CheckNonAscii(input_api, output_api):
  """Check for non-ASCII characters and throw warnings if any are found."""
  results = []
  files_with_unicode_lines = []
  # We keep track of the longest file (in line count) so that we can pad the
  # numbers when displaying output. This makes it easier to see the indention.
  max_lines_in_any_file = 0
  FILE_EXTENSIONS = ['bat', 'cfg', 'cmd', 'conf', 'css', 'gyp', 'gypi', 'htm',
                     'html', 'js', 'json', 'ps1', 'py', 'sh', 'tac', 'yaml']
  file_filter = _MakeFileFilter(input_api, FILE_EXTENSIONS)
  for affected_file in input_api.AffectedSourceFiles(file_filter):
    affected_filepath = affected_file.LocalPath()
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


def _CheckBannedGoAPIs(input_api, output_api):
  """Check go source code for functions and packages that should not be used."""
  # TODO(benjaminwagner): A textual search is easy, but it would be more
  #   accurate to parse and analyze the source due to package aliases.
  # A list of tuples of a regex to match an API and a suggested replacement for
  # that API.
  banned_replacements = [
    (r'\breflect\.DeepEqual\b', 'DeepEqual in go.skia.org/infra/go/testutils'),
    (r'\bgithub\.com/golang/glog\b', 'go.skia.org/infra/go/sklog'),
    (r'\bgithub\.com/skia-dev/glog\b', 'go.skia.org/infra/go/sklog'),
    (r'\bhttp\.Get\b', 'NewTimeoutClient in go.skia.org/infra/go/httputils'),
    (r'\bhttp\.Head\b', 'NewTimeoutClient in go.skia.org/infra/go/httputils'),
    (r'\bhttp\.Post\b', 'NewTimeoutClient in go.skia.org/infra/go/httputils'),
    (r'\bhttp\.PostForm\b',
        'NewTimeoutClient in go.skia.org/infra/go/httputils'),
    (r'\bos\.Interrupt\b', 'AtExit in go.skia.org/go/cleanup'),
    (r'\bsignal\.Notify\b', 'AtExit in go.skia.org/go/cleanup'),
    (r'\bsyscall.SIGINT\b', 'AtExit in go.skia.org/go/cleanup'),
    (r'\bsyscall.SIGTERM\b', 'AtExit in go.skia.org/go/cleanup'),
    (r'\bsyncmap.Map\b', 'sync.Map, added in go 1.9'),
    (r'assert\s+"github\.com/stretchr/testify/require"',
     'non-aliased import; this can be confused with package ' +
         '"github.com/stretchr/testify/assert"'),
    (r'"git"', 'Executable in go.skia.org/infra/go/git', [
      # These don't actually shell out to git; the tests look for "git" in the
      # command line and mock stdout accordingly.
      r'autoroll/go/repo_manager/.*_test.go',
      # This doesn't shell out to git; it's referring to a CIPD package with
      # the same name.
      r'infra/bots/gen_tasks.go',
      # This is the one place where we are allowed to shell out to git; all
      # others should go through here.
      r'go/git/git_common/.*.go',
    ]),
  ]

  compiled_replacements = []
  for rep in banned_replacements:
    exceptions = []
    if len(rep) == 3:
      (re, replacement, exceptions) = rep
    else:
      (re, replacement) = rep

    compiled_re = input_api.re.compile(re)
    compiled_exceptions = [input_api.re.compile(exc) for exc in exceptions]
    compiled_replacements.append(
        (compiled_re, replacement, compiled_exceptions))

  errors = []
  file_filter = _MakeFileFilter(input_api, ['go'])
  for affected_file in input_api.AffectedSourceFiles(file_filter):
    affected_filepath = affected_file.LocalPath()
    for (line_num, line) in affected_file.ChangedContents():
      for (re, replacement, exceptions) in compiled_replacements:
        match = re.search(line)
        if match:
          for exc in exceptions:
            if exc.search(affected_filepath):
              break
          else:
            errors.append('%s:%s: Instead of %s, please use %s.' % (
                affected_filepath, line_num, match.group(), replacement))

  if errors:
    return [output_api.PresubmitPromptWarning('\n'.join(errors))]

  return []

def _CheckJSDebugging(input_api, output_api):
  """Check JS source code for left over testing/debugging artifacts."""
  to_warn_regexes = [
    input_api.re.compile('debugger;'),
    input_api.re.compile('it\\.only\\('),
    input_api.re.compile('describe\\.only\\('),
  ]
  errors = []
  file_filter = _MakeFileFilter(input_api, ['js', 'ts'])
  for affected_file in input_api.AffectedSourceFiles(file_filter):
      affected_filepath = affected_file.LocalPath()
      for (line_num, line) in affected_file.ChangedContents():
          for re in to_warn_regexes:
              match = re.search(line)
              if match:
                  errors.append('%s:%s: JS debugging code found (%s)' % (
                      affected_filepath, line_num, match.group()))

  if errors:
      return [output_api.PresubmitPromptWarning('\n'.join(errors))]

  return []

def _RunCommandAndCheckGitDiff(input_api, output_api, command):
  """Run an arbitrary command. Fail if it produces any diffs."""
  command_str = ' '.join(command)
  results = []

  print('Running "%s" ...' % command_str)
  try:
    input_api.subprocess.check_output(
        command,
        stderr=input_api.subprocess.STDOUT)
  except input_api.subprocess.CalledProcessError as e:
    results += [output_api.PresubmitError(
        'Command "%s" returned non-zero exit code %d. Output: \n\n%s' % (
            command_str,
            e.returncode,
            e.output,
        )
    )]

  git_diff_output = input_api.subprocess.check_output(
      ['git', 'diff', '--no-ext-diff'])
  if git_diff_output:
    results += [output_api.PresubmitError(
        'Diffs found after running "%s":\n\n%s\n'
        'Please commit or discard the above changes.' % (
            command_str,
            git_diff_output,
        )
    )]

  return results

def _CheckBuildifier(input_api, output_api):
  """Runs Buildifier and fails on linting errors, or if it produces any diffs.

  This check only runs if the affected files include any WORKSPACE, BUILD,
  BUILD.bazel or *.bzl files.
  """
  file_filter = _MakeFileFilter(
      input_api,
      include_filenames=['WORKSPACE', 'BUILD', 'BUILD.bazel'],
      include_extensions=['bzl'])
  if not input_api.AffectedSourceFiles(file_filter):
    return []
  return _RunCommandAndCheckGitDiff(
      input_api, output_api, ['bazel', 'run', '//:buildifier'])

def _CheckGazelle(input_api, output_api):
  """Runs Gazelle and fails if it produces any diffs.

  This check only runs if the affected files include any *.go, *.ts, WORKSPACE,
  BUILD, BUILD.bazel or *.bzl files.

  WORKSPACE and *.bzl files are included in the above list because some such
  changes may affect Gazelle's behavior.
  """
  file_filter = _MakeFileFilter(
      input_api,
      include_filenames=['WORKSPACE', 'BUILD', 'BUILD.bazel'],
      include_extensions=['go', 'ts', 'bzl'])
  if not input_api.AffectedSourceFiles(file_filter):
    return []
  return _RunCommandAndCheckGitDiff(input_api, output_api, ['make', 'gazelle'])

def _CheckGoFmt(input_api, output_api):
  """Runs gofmt and fails if it producess any diffs.

  This check only runs if the affected files include any *.go files.
  """
  if not input_api.AffectedSourceFiles(_MakeFileFilter(input_api, ['go'])):
    return []
  return _RunCommandAndCheckGitDiff(
      input_api, output_api, ['gofmt', '-s', '-w', '.'])

def CheckChange(input_api, output_api):
  """Presubmit checks for the change on upload or commit.

  The presubmit checks have been handpicked from the list of canned checks
  here:
  https://chromium.googlesource.com/chromium/tools/depot_tools/+show/master/presubmit_canned_checks.py

  The following are the presubmit checks:
  * Pylint is run if the change contains any .py files.
  * Enforces max length for all lines is 100.
  * Checks that the user didn't add TODO(name) without an owner.
  * Checks that there is no stray whitespace at source lines end.
  * Checks that there are no tab characters in any of the text files.
  * No banned go apis (suggesting alternatives)
  * No JS debugging artifacts.
  * No Buildifier diffs.
  * No Gazelle diffs.
  * No gmfmt diffs.
  """
  results = []

  pylint_skip = [
      r'infra[\\\/]bots[\\\/]recipes.py',
      r'.*[\\\/]\.recipe_deps[\\\/].*',
      r'.*[\\\/]node_modules[\\\/].*',
  ]
  pylint_skip.extend(input_api.DEFAULT_FILES_TO_SKIP)
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
  results += input_api.canned_checks.RunPylint(
      input_api, output_api,
      disabled_warnings=pylint_disabled_warnings,
      files_to_skip=pylint_skip)

  # Use 100 for max length for files other than python. Python length is
  # already checked during the Pylint above. No max length for Go files.
  IGNORE_LINE_LENGTH_EXTS = ['go', 'html', 'py']
  IGNORE_LINE_LENGTH_FILENAMES = ['package-lock.json']
  file_filter = _MakeFileFilter(input_api,
                                exclude_extensions=IGNORE_LINE_LENGTH_EXTS,
                                exclude_filenames=IGNORE_LINE_LENGTH_FILENAMES)
  results += input_api.canned_checks.CheckLongLines(input_api, output_api, 100,
      source_file_filter=file_filter)

  file_filter = _MakeFileFilter(input_api)
  results += input_api.canned_checks.CheckChangeTodoHasOwner(
      input_api, output_api, source_file_filter=file_filter)
  results += input_api.canned_checks.CheckChangeHasNoStrayWhitespace(
      input_api, output_api, source_file_filter=file_filter)

  # CheckChangeHasNoTabs automatically ignores makefiles and golang files.
  results += input_api.canned_checks.CheckChangeHasNoTabs(input_api, output_api)

  results += _CheckBannedGoAPIs(input_api, output_api)
  results += _CheckJSDebugging(input_api, output_api)
  results += _CheckBuildifier(input_api, output_api)
  results += _CheckGazelle(input_api, output_api)
  results += _CheckGoFmt(input_api, output_api)

  if input_api.is_committing:
    results.extend(input_api.canned_checks.CheckDoNotSubmitInDescription(
        input_api, output_api))
    results.extend(input_api.canned_checks.CheckDoNotSubmitInFiles(
        input_api, output_api))

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

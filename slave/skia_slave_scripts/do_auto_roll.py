#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the Blink AutoRoll bot for Skia."""


import os
import re
import sys

from build_step import BuildStep
from slave import slave_utils
from utils import misc
from utils import shell_utils

sys.path.append(misc.BUILDBOT_PATH)

from site_config import skia_vars


DEPS_ROLL_AUTHOR = 'skia-deps-roller@chromium.org'
DEPS_ROLL_NAME = 'Skia DEPS Roller'
HTML_CONTENT = '''
<html>
<head>
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="-1">
<meta http-equiv="refresh" content="0; url=%s" />
</head>
</html>
'''
ISSUE_URL_TEMPLATE = 'https://codereview.chromium.org/%(issue)s/'

# TODO(borenet): Find a way to share these filenames (or their full GS URL) with
# the webstatus which links to them.
FILENAME_CURRENT_ATTEMPT = 'depsroll.html'
FILENAME_ROLL_STATUS = 'arb_status.html'

REGEXP_ISSUE_CREATED = (
    r'Issue created. URL: https://codereview.chromium.org/(?P<issue>\d+)')
REGEXP_ROLL_ACTIVE = (
    r'https://codereview.chromium.org/(?P<issue>\d+)/ is still active')
REGEXP_ROLL_STOPPED = (
    r'https://codereview.chromium.org/(?P<issue>\d+)/: Rollbot was stopped by')
# This occurs when the ARB has "caught up" and has nothing new to roll, or when
# a different roll (typically a manual roll) has already rolled past it.
REGEXP_ROLL_TOO_OLD = r'Already at .+ refusing to roll backwards to .+'

ROLL_STATUS_IN_PROGRESS = 'In progress - %s' % ISSUE_URL_TEMPLATE
ROLL_STATUS_STOPPED = 'Stopped - %s' % ISSUE_URL_TEMPLATE
ROLL_STATUS_IDLE = 'Idle'

ROLL_STATUSES = [
    (REGEXP_ISSUE_CREATED, ROLL_STATUS_IN_PROGRESS),
    (REGEXP_ROLL_ACTIVE,   ROLL_STATUS_IN_PROGRESS),
    (REGEXP_ROLL_STOPPED,  ROLL_STATUS_STOPPED),
    (REGEXP_ROLL_TOO_OLD,  ROLL_STATUS_IDLE),
]


class AutoRoll(BuildStep):
  """BuildStep which runs the Blink AutoRoll bot."""

  def _Run(self):
    shell_utils.run(['git', 'config', '--local', 'user.name', DEPS_ROLL_NAME])
    shell_utils.run(['git', 'config', '--local', 'user.email',
                     DEPS_ROLL_AUTHOR])
    auto_roll = os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot_tot', 'scripts', 'tools',
                             'blink_roller', 'auto_roll.py')
    chrome_path = os.path.join(os.pardir, 'src')
    # python auto_roll.py <project> <author> <path to chromium/src>
    cmd = ['python', auto_roll, 'skia', DEPS_ROLL_AUTHOR, chrome_path]

    exception = None
    try:
      output = shell_utils.run(cmd)
    except shell_utils.CommandFailedException as e:
      output = e.output
      exception = e

    match = re.search(REGEXP_ISSUE_CREATED, output)
    if match:
      issue = match.group('issue')
      print 'Found issue #', issue
      with open(FILENAME_CURRENT_ATTEMPT, 'w') as f:
        f.write(HTML_CONTENT % (ISSUE_URL_TEMPLATE % {'issue': issue}))
      slave_utils.GSUtilCopyFile(
          filename=FILENAME_CURRENT_ATTEMPT,
          gs_base=skia_vars.GetGlobalVariable('googlestorage_bucket'),
          subdir=None,
          gs_acl='public-read')

    roll_status = None
    for regexp, status_msg in ROLL_STATUSES:
      match = re.search(regexp, output)
      if match:
        roll_status = status_msg % match.groupdict()
        break

    if roll_status:
      with open(FILENAME_ROLL_STATUS, 'w') as f:
        f.write(roll_status)
      slave_utils.GSUtilCopyFile(
          filename=FILENAME_ROLL_STATUS,
          gs_base=skia_vars.GetGlobalVariable('googlestorage_bucket'),
          subdir=None,
          gs_acl='public-read')

    #pylint: disable=E0702
    if exception:
      raise exception


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AutoRoll))

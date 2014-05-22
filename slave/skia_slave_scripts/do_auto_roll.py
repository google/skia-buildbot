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


# TODO(borenet): Set up an automated account for this:
# https://code.google.com/p/chromium/issues/detail?id=339824
DEPS_ROLL_AUTHOR = 'robertphillips@google.com'
HTML_CONTENT = '''
<html>
<head>
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="-1">
<meta http-equiv="refresh" content="0; url=https://codereview.chromium.org/%s/" />
</head>
</html>
'''
ISSUE_REGEXP = (
    r'Issue created. URL: https://codereview.chromium.org/(?P<issue>\d+)')
UPLOAD_FILENAME = 'depsroll.html'


class AutoRoll(BuildStep):
  """BuildStep which runs the Blink AutoRoll bot."""

  def _Run(self):
    auto_roll = os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot_tot', 'scripts', 'tools',
                             'blink_roller', 'auto_roll.py')
    chrome_path = os.path.join(os.pardir, 'src')
    # python auto_roll.py <project> <author> <path to chromium/src>
    cmd = ['python', auto_roll, 'skia', DEPS_ROLL_AUTHOR, chrome_path]
    output = shell_utils.run(cmd)

    match = re.search(ISSUE_REGEXP, output)
    if match:
      issue = match.group('issue')
      print 'Found issue #', issue
      with open(UPLOAD_FILENAME, 'w') as f:
        f.write(HTML_CONTENT % issue)
      slave_utils.GSUtilCopyFile(
          filename=UPLOAD_FILENAME,
          gs_base=skia_vars.GetGlobalVariable('googlestorage_bucket'),
          subdir=None,
          gs_acl='public-read')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(AutoRoll))

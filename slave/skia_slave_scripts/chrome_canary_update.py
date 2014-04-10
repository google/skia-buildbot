#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """

# build_step must be imported first, since it does some tweaking of PYTHONPATH.
from build_step import BuildStep
from utils import gclient_utils
from utils import shell_utils
from utils import sync_skia_in_chrome
import shlex
import sys


CHROMIUM_REPO = 'https://chromium.googlesource.com/chromium/src.git'


class ChromeCanaryUpdate(BuildStep):
  def __init__(self, timeout=24000, no_output_timeout=4800, attempts=5,
               **kwargs):
    super(ChromeCanaryUpdate, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        attempts=attempts,
        **kwargs)

  def _Run(self):
    if 'ToT' in self._builder_name:
      chrome_rev = shlex.split(shell_utils.run(
          [gclient_utils.GIT, 'ls-remote', CHROMIUM_REPO,
           'refs/heads/master']))[0]
    else:
      chrome_rev = self._args.get('chrome_rev')

    override_skia_checkout = True
    if 'AutoRoll' in self._builder_name:
      override_skia_checkout = False

    (got_skia_rev, got_chrome_rev) = sync_skia_in_chrome.Sync(
        skia_revision=self._revision,
        chrome_revision=chrome_rev,
        use_lkgr_skia=('use_lkgr_skia' in self._args.keys()),
        override_skia_checkout=override_skia_checkout)
    print 'Skia updated to %s' % got_skia_rev
    print 'Chrome updated to %s' % got_chrome_rev


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeCanaryUpdate))

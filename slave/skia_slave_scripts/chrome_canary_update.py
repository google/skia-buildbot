#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """

# build_step must be imported first, since it does some tweaking of PYTHONPATH.
from build_step import BuildStep
from utils import sync_skia_in_chrome
import sys


class ChromeCanaryUpdate(BuildStep):
  def __init__(self, timeout=24000, no_output_timeout=4800, attempts=5,
               **kwargs):
    super(ChromeCanaryUpdate, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        attempts=attempts,
        **kwargs)

  def _Run(self):
    chrome_rev = self._args.get('chrome_rev',
                                sync_skia_in_chrome.CHROME_REV_MASTER)
    skia_rev = (sync_skia_in_chrome.SKIA_REV_DEPS
                    if self._args.get('use_lkgr_skia')
                else (self._revision or sync_skia_in_chrome.SKIA_REV_MASTER))
    (got_skia_rev, got_chrome_rev) = sync_skia_in_chrome.Sync(
        skia_revision=skia_rev,
        chrome_revision=chrome_rev,
        gyp_defines=self._flavor_utils.gyp_defines,
        gyp_generators=self._flavor_utils.gyp_generators)
    print 'Skia updated to %s' % got_skia_rev
    print 'Chrome updated to %s' % got_chrome_rev


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeCanaryUpdate))

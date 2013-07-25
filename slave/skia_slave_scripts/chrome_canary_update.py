#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Check out the Skia sources. """


from utils import sync_skia_in_chrome
from build_step import BuildStep
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
    (skia_rev, chrome_rev) = \
        sync_skia_in_chrome.Sync(skia_revision=self._revision)
    print 'Skia updated to revision %s' % skia_rev
    print 'Chrome updated to revision %s' % chrome_rev


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeCanaryUpdate))

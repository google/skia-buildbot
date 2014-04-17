#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Download expectations for render_pictures.py.

TODO(epoger): Delete this step; expectations will be handled similarly to GM,
and they can be prepared within prerender.py just like the GM expectations.
"""

import sys

from build_step import BuildStep


class DownloadBaselines(BuildStep):

  def _Run(self):
    # Do nothing.  This step will soon be deleted.
    return


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DownloadBaselines))

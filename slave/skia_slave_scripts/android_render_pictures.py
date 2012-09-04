#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import misc
from build_step import BuildStep
import os
import shutil
import sys
import tempfile

class RenderPictures(BuildStep):
  def _Run(self, args):
    # Do nothing until we can run render_pictures on Android.
    pass

if '__main__' == __name__:
  sys.exit(BuildStep.Run(RenderPictures))


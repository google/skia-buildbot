#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Launch render_skps.py.

TODO(epoger): Once the master is calling render_skps.py directly, we can
delete this file.
"""

from build_step import BuildStep
import sys
import render_skps

if '__main__' == __name__:
  print 'Chaining to render_skps.py'
  sys.exit(BuildStep.RunBuildStep(render_skps.RenderSKPs))

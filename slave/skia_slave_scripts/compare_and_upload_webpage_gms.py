#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Launch upload_rendered_skps.py.

TODO(epoger): Once the master is calling upload_rendered_skps.py directly, we
can delete this file.
"""

from build_step import BuildStep
import sys
import upload_rendered_skps

if '__main__' == __name__:
  print 'Chaining to upload_rendered_skps.py'
  sys.exit(BuildStep.RunBuildStep(upload_rendered_skps.UploadRenderedSKPs))

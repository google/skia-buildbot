#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compares the results of render_skps.py against expectations.
"""

import sys

from build_step import BuildStep
import upload_rendered_skps


class CompareRenderedSKPs(BuildStep):

  def _Run(self):
    print ('To view the latest SKP renderings by this builder, see:\n%s' %
           upload_rendered_skps.rebaseline_server_url(
               directive='static/live-view.html#/live-view.html?',
               builder_name=self._builder_name))
    print ''
    print 'TODO(epoger): Compare not yet implemented; see http://skbug.com/1942'


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareRenderedSKPs))

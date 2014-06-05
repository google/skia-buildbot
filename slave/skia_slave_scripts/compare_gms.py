#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compare the generated GM images to the baselines """

# System-level imports
import os
import sys

from build_step import BuildStep, BuildStepWarning
from utils import misc
import run_gm
import skia_vars

LIVE_REBASELINE_SERVER_BASEURL = (
    'http://skia-tree-status.appspot.com/redirect/rebaseline-server/'
    'static/view.html#/view.html')


class CompareGMs(BuildStep):
  def _Run(self):
    json_summary_path = misc.GetAbsPath(os.path.join(
        self._gm_actual_dir, run_gm.JSON_SUMMARY_FILENAME))

    # Temporary list of builders who are allowed to fail this step without the
    # bot turning red.
    may_fail_with_warning = []
    # This import must happen after BuildStep.__init__ because it requires that
    # CWD is in PYTHONPATH, and BuildStep.__init__ may change the CWD.
    from gm import display_json_results
    success = display_json_results.Display(json_summary_path)
    print ('%s<a href="%s?resultsToLoad=/results/failures&builder=%s">'
           'link</a>' % (
               skia_vars.GetGlobalVariable('latest_gm_failures_preamble'),
               LIVE_REBASELINE_SERVER_BASEURL, self._builder_name))
    if not success:
      if self._builder_name in may_fail_with_warning:
        raise BuildStepWarning('Expectations mismatch in %s!' %
                               json_summary_path)
      else:
        raise Exception('Expectations mismatch in %s!' % json_summary_path)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareGMs))

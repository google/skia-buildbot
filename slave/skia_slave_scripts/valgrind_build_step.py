# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Valgrind build steps. """

from build_step import BuildStep
from flavor_utils import valgrind_build_step_utils


class ValgrindBuildStep(BuildStep):
  def __init__(self, suppressions_file=None, **kwargs):
    self._suppressions_file = suppressions_file
    super(ValgrindBuildStep, self).__init__(timeout=12000,
                                            no_output_timeout=9600,**kwargs)
    self._flavor_utils = valgrind_build_step_utils.ValgrindBuildStepUtils(self)

  @property
  def suppressions_file(self):
    return self._suppressions_file

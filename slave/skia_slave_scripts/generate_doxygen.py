#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Generate Doxygen documentation."""

import os
import sys

from build_step import BuildStep
from utils import file_utils, shell_utils

DOXYGEN_WORKING_DIR = os.path.join(os.pardir, os.pardir, 'doxygen-contents')


class GenerateDoxygen(BuildStep):
  def _Run(self):
    file_utils.create_clean_local_dir(DOXYGEN_WORKING_DIR)

    os.environ['DOXYGEN_TEMPDIR'] = DOXYGEN_WORKING_DIR
    os.environ['DOXYGEN_COMMIT'] = 'false'
    update_doxygen_path = os.path.join('tools', 'update-doxygen.sh')
    shell_utils.run(update_doxygen_path)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(GenerateDoxygen))

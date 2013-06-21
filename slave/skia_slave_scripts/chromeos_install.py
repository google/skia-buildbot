#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install all executables, and any runtime resources that are needed by
    *both* Test and Bench builders. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from install import Install
from utils import ssh_utils
import os
import sys


class ChromeOSInstall(ChromeOSBuildStep, Install):
  def _PutSCP(self, executable):
    ssh_utils.PutSCP(# TODO(borenet): Use the new chromeos_make script.
                     #local_path=os.path.join('out', 'config',
                     #                        'chromeos-' + self._args['board'],
                     #                        self._configuration, executable),
                     local_path=self._PathToBinary(executable),
                     remote_path='/usr/local/bin/skia_%s' % executable,
                     username=self._ssh_username,
                     host=self._ssh_host,
                     port=self._ssh_port)

  def _Run(self):
    super(ChromeOSInstall, self)._Run()

    self._PutSCP('tests')
    self._PutSCP('gm')
    self._PutSCP('render_pictures')
    self._PutSCP('render_pdfs')
    self._PutSCP('bench')
    self._PutSCP('bench_pictures')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSInstall))

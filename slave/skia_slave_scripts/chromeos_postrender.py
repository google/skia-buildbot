#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Pulls the directory with render results from the ChomeOS device. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from utils import ssh_utils
import posixpath
import shlex
import sys


class ChromeOSPostRender(ChromeOSBuildStep):
  def __init__(self, args, attempts=1, timeout=4800, **kwargs):
    super(ChromeOSPostRender, self).__init__(args, attempts=attempts,
                                             timeout=timeout, **kwargs)


  def _PullSKPResults(self):
    img_list = shlex.split(ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
        self._ssh_port, ['ls', self._device_dirs.SKPOutDir()], echo=False))
    for img in img_list:
      ssh_utils.GetSCP(self._gm_actual_dir,
                       posixpath.join(self._device_dirs.SKPOutDir(), img),
                       self._ssh_username, self._ssh_host, self._ssh_port)

  def _Run(self):
    self._PullSKPResults()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSPostRender))


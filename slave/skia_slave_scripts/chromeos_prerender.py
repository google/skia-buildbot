#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Prepare runtime resources that are needed by Test builders but not
    Bench builders. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from prerender import PreRender
from utils import ssh_utils
import posixpath
import sys


class ChromeOSPreRender(ChromeOSBuildStep, PreRender):
  def __init__(self, args, attempts=1, timeout=4800, **kwargs):
    super(ChromeOSPreRender, self).__init__(args, attempts=attempts,
                                            timeout=timeout, **kwargs)

  def _Run(self):
    super(ChromeOSPreRender, self)._Run()

    # Clear the GM directory on the device.
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', posixpath.join(self._device_dirs.GMDir(),
                                                    self._gm_image_subdir)])
    except Exception:
      pass
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPOutDir()])
    except Exception:
      pass
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p', posixpath.join(self._device_dirs.GMDir(),
                                                    self._gm_image_subdir)])
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p',
                      posixpath.join(self._device_dirs.SKPOutDir())])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSPreRender))

#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Push the skp directory full of .skp's to the ChromeOS device. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from utils import ssh_utils
import os
import posixpath
import sys


class ChromeOSPreRender(ChromeOSBuildStep):
  def __init__(self, args, attempts=1, timeout=4800, **kwargs):
    super(ChromeOSPreRender, self).__init__(args, attempts=attempts,
                                            timeout=timeout, **kwargs)

  def _PushSKPSources(self):
    """ Push the skp directory full of .skp's to the ChromeoS device. """
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPDir()])
    except Exception:
      pass
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPOutDir()])
    except Exception:
      pass
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p', self._device_dirs.SKPDir()])
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p',
                      posixpath.join(self._device_dirs.SKPOutDir())])
    skp_list = os.listdir(self._skp_dir)
    for skp in skp_list:
      if os.path.isfile(os.path.join(self._skp_dir, skp)):
        ssh_utils.PutSCP(os.path.join(self._skp_dir, skp),
                         self._device_dirs.SKPDir(), self._ssh_username,
                         self._ssh_host, self._ssh_port)

  def _Run(self):
    self._PushSKPSources()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSPreRender))


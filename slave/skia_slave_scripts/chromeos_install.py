#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Install the Skia executables. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from install import Install
from utils import ssh_utils
import os
import sys


class ChromeOSInstall(ChromeOSBuildStep, Install):
  def _PutSCP(self, executable):
    ssh_utils.PutSCP(local_path=self._PathToBinary(executable),
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

    # Push the SKPs to the device.
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPDir()])
    except Exception:
      pass
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p', self._device_dirs.SKPDir()])
    skp_list = os.listdir(self._skp_dir)
    for skp in skp_list:
      if os.path.isfile(os.path.join(self._skp_dir, skp)):
        ssh_utils.PutSCP(os.path.join(self._skp_dir, skp),
                         self._device_dirs.SKPDir(), self._ssh_username,
                         self._ssh_host, self._ssh_port)

    # Push the GM expectations to the device.
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.GMExpectedDir()])
    except Exception:
      pass
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['mkdir', '-p', self._device_dirs.GMExpectedDir()])
    # TODO(borenet) Enable expectations once we're using checksums.  It will
    # take too long to push the expected images, but the checksums will be
    # much faster.
    #expectation_list = os.listdir(self._gm_expected_dir)
    #for e in expectation_list:
    #  if os.path.isfile(os.path.join(self._gm_expected_dir, e)):
    #    ssh_utils.PutSCP(os.path.join(self._gm_expected_dir, e),
    #                     self._device_dirs.GMExpectedDir(),
    #                     self._ssh_username, self._ssh_host, self._ssh_port)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSInstall))

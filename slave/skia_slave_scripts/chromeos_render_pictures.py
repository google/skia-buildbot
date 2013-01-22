#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from chromeos_build_step import ChromeOSBuildStep
from build_step import BuildStep
from render_pictures import RenderPictures
from utils import ssh_utils
import glob
import os
import posixpath
import shlex
import shutil
import sys


BINARY_NAME = 'skia_render_pictures'


class ChromeOSRenderPictures(RenderPictures, ChromeOSBuildStep):
  def __init__(self, timeout=4800, **kwargs):
    super(ChromeOSRenderPictures, self).__init__(timeout=timeout, **kwargs)

  def _PushSKPSources(self):
    """ Push the skp directory full of .skp's to the ChromeoS device. """
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPDir()])
    except:
      pass
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.SKPOutDir()])
    except:
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

  def _PullSKPResults(self):
    img_list = shlex.split(ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
        self._ssh_port, ['ls', self._device_dirs.SKPOutDir()], echo=False))
    for img in img_list:
      ssh_utils.GetSCP(self._gm_actual_dir,
                       posixpath.join(self._device_dirs.SKPOutDir(), img),
                       self._ssh_username, self._ssh_host, self._ssh_port)

  def DoRenderPictures(self, verify_args):
    args = self._PictureArgs(self._device_dirs.SKPDir(),
                             self._device_dirs.SKPOutDir(), 'bitmap')
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     [BINARY_NAME] + args + verify_args)

  def _Run(self):
    # For this step, we assume that we run *after* RunGM and *before*
    # UploadGMResults.  This needs to be the case, because RunGM clears the
    # output directory before it begins, and because we want the results from
    # this step to be uploaded with the GM results.
    self._PushSKPSources()
    super(ChromeOSRenderPictures, self)._Run()
    self._PullSKPResults()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRenderPictures))


# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side ChromeOS build steps. """


from build_step import BuildStep, DeviceDirs
from utils import ssh_utils


class ChromeOSBuildStep(BuildStep):
  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['skia_%s' % app] + args)

  def __init__(self, args, **kwargs):
    self._ssh_host = args['ssh_host']
    self._ssh_port = args['ssh_port']
    self._ssh_username = 'root'
    super(ChromeOSBuildStep, self).__init__(args=args, **kwargs)
    prefix = '/usr/local/skiabot/skia_'
    self._device_dirs = DeviceDirs(perf_data_dir=prefix + 'perf',
                                   gm_dir=prefix + 'gm',
                                   skp_dir=prefix + 'skp',
                                   skp_perf_dir=prefix + 'skp_perf',
                                   skp_out_dir=prefix + 'skp_out')

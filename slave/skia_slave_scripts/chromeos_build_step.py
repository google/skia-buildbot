# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side ChromeOS build steps. """


from build_step import BuildStep, DeviceDirs
from flavor_utils import chromeos_build_step_utils


class ChromeOSBuildStep(BuildStep):
  def __init__(self, args, **kwargs):
    self._ssh_host = args['ssh_host']
    self._ssh_port = args['ssh_port']
    self._ssh_username = 'root'
    super(ChromeOSBuildStep, self).__init__(args=args, **kwargs)
    self._flavor_utils = chromeos_build_step_utils.ChromeOSBuildStepUtils(self)
    prefix = '/usr/local/skiabot/skia_'
    self._device_dirs = DeviceDirs(perf_data_dir=prefix + 'perf',
                                   gm_actual_dir=prefix + 'gm_actual',
                                   gm_expected_dir=prefix + 'gm_expected',
                                   resource_dir=prefix + 'resources',
                                   skimage_in_dir=prefix + 'skimage_in',
                                   skimage_expected_dir=(prefix
                                                         + 'skimage_expected'),
                                   skimage_out_dir=prefix + 'skimage_out',
                                   skp_dir=prefix + 'skp',
                                   skp_perf_dir=prefix + 'skp_perf',
                                   skp_out_dir=prefix + 'skp_out',
                                   tmp_dir=prefix + 'tmp_dir')

  @property
  def ssh_username(self):
    return self._ssh_username

  @property
  def ssh_host(self):
    return self._ssh_host

  @property
  def ssh_port(self):
    return self._ssh_port

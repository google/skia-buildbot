# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Android build steps. """

from build_step import BuildStep, DeviceDirs
from flavor_utils import android_build_step_utils
from utils import android_utils
from utils import shell_utils
import posixpath


class AndroidBuildStep(BuildStep):
  def _PreRun(self):
    if self.serial:
      if self.has_root:
        android_utils.RunADB(self.serial, ['root'])
        android_utils.RunADB(self.serial, ['remount'])
        android_utils.SetCPUScalingMode(self.serial, 'performance')
        android_utils.ADBKill(self.serial, 'skia')
      else:
        android_utils.ADBKill(self.serial, 'com.skia', kill_app=True)

  def __init__(self, args, **kwargs):
    self._device = args['device']
    self._serial = args['serial']
    self._has_root = args['has_root'] == 'True'
    if self._serial == 'None':
      self._serial = None
      print 'WARNING: No device serial number provided!'
    else:
      device_scratch_dir = shell_utils.Bash(
          '%s -s %s shell echo \$EXTERNAL_STORAGE' % (
              android_utils.PATH_TO_ADB, self._serial),
          echo=True, shell=True).rstrip().split('\n')[-1]
    super(AndroidBuildStep, self).__init__(args=args, **kwargs)
    self._flavor_utils = android_build_step_utils.AndroidBuildStepUtils(self)
    if self._serial:
      prefix = posixpath.join(device_scratch_dir, 'skiabot', 'skia_')
      self._device_dirs = DeviceDirs(perf_data_dir=prefix + 'perf',
                                     gm_actual_dir=prefix + 'gm_actual',
                                     gm_expected_dir=prefix + 'gm_expected',
                                     resource_dir=prefix + 'resources',
                                     skimage_in_dir=prefix + 'skimage_in',
                                     skimage_expected_dir=(prefix +
                                                           'skimage_expected'),
                                     skimage_out_dir=prefix + 'skimage_out',
                                     skp_dir=prefix + 'skp',
                                     skp_perf_dir=prefix + 'skp_perf',
                                     skp_out_dir=prefix + 'skp_out',
                                     tmp_dir=prefix + 'tmp_dir')

  @property
  def serial(self):
    return self._serial

  @property
  def has_root(self):
    return self._has_root

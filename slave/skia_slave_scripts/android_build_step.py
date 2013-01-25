# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Android build steps. """

from build_step import BuildStep
from utils import android_utils
from utils import shell_utils


class AndroidDirs(object):
  def __init__(self, path_prefix):
    self._path_prefix = path_prefix + '/skiabot/skia_'

  def GMDir(self):
    return self._path_prefix + 'gm'

  def PerfDir(self):
    return self._path_prefix + 'perf'

  def SKPDir(self):
    return self._path_prefix + 'skp'

  def SKPPerfDir(self):
    return self._path_prefix + 'skp_perf'

  def SKPOutDir(self):
    return self._path_prefix + 'skp_out'


class AndroidBuildStep(BuildStep):
  def _PreRun(self):
    android_utils.RunADB(self._serial, ['root'])
    android_utils.RunADB(self._serial, ['remount'])
    android_utils.SetCPUScalingMode(self._serial, 'performance')
    android_utils.ADBKill(self._serial, 'skia')

  def __init__(self, args, **kwargs):
    self._device = args['device']
    self._serial = args['serial']
    if self._serial == 'None':
      self._serial = android_utils.GetSerial(self._device)
    device_scratch_dir = shell_utils.Bash(
        '%s -s %s shell echo \$EXTERNAL_STORAGE' % (
            android_utils.PATH_TO_ADB, self._serial), 
        echo=True, shell=True).rstrip().split('\n')[-1]
    self._device_dirs = AndroidDirs(device_scratch_dir)
    super(AndroidBuildStep, self).__init__(args=args, **kwargs)

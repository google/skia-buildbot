# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Android build steps. """

from build_step import BuildStep
from utils import misc

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
    misc.RunADB(self._serial, ['root'])
    misc.RunADB(self._serial, ['remount'])
    misc.SetCPUScalingMode(self._serial, 'performance')

  def __init__(self, args, attempts=1):
    self._device = args['device']
    self._serial = args['serial']
    if self._serial == 'None':
      self._serial = misc.GetSerial(self._device)
    # ADB has a different view of the device filesystem than the Android app,
    # so we need to use two paths to 'skiabot' and its subdirectories.
    adb_scratch_dir = misc.Bash("%s -s %s shell echo \$EXTERNAL_STORAGE" % (
                                       misc.PATH_TO_ADB, self._serial), 
                                echo=True).rstrip()
    self._app_dirs = AndroidDirs('/sdcard')
    self._adb_dirs = AndroidDirs(adb_scratch_dir)
    super(AndroidBuildStep, self).__init__(args, attempts)
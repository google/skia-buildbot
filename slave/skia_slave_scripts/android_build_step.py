# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side Android build steps. """

from build_step import BuildStep, DeviceDirs
from utils import android_utils
from utils import gs_utils
from utils import shell_utils
import os
import posixpath


class AndroidBuildStep(BuildStep):
  def _PreRun(self):
    if self._has_root:
      android_utils.RunADB(self._serial, ['root'])
      android_utils.RunADB(self._serial, ['remount'])
      android_utils.SetCPUScalingMode(self._serial, 'performance')
      android_utils.ADBKill(self._serial, 'skia')
    else:
      android_utils.ADBKill(self._serial, 'com.skia', kill_app=True)

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    android_utils.RunSkia(self._serial, [app] + args,
                          use_intent=(not self._has_root),
                          stop_shell=self._has_root)

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the device. """
    return android_utils.ADBShell(self._serial, ['cat', filepath, ';', 'echo'],
                                  echo=False)

  def _RemoveDirectoryOnDevice(self, directory):
    """ Delete a directory on the device. """
    try:
      android_utils.RunADB(self._serial, ['shell', 'rm', '-r', directory])
    except Exception:
      pass
    if 'DIRECTORY_EXISTS' in android_utils.ADBShell(self._serial,
        ['if', '[', '-d', directory, '];', 'then', 'echo', 'DIRECTORY_EXISTS;',
         'fi']):
      raise Exception('Failed to remove %s' % directory)

  def _CreateDirectoryOnDevice(self, directory):
    """ Create a directory on the device. """
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p', directory])

  def PushFileToDevice(self, src, dst):
    """ Copy a file to the device. """
    android_utils.RunADB(self._serial, ['push', src, dst])

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side.
    """
    self._RemoveDirectoryOnDevice(device_dir)
    self._CreateDirectoryOnDevice(device_dir)
    file_list = os.listdir(host_dir)
    for f in file_list:
      if f == gs_utils.TIMESTAMP_COMPLETED_FILENAME:
        continue
      if os.path.isfile(os.path.join(host_dir, f)):
        self.PushFileToDevice(os.path.join(host_dir, f), device_dir)
    ts_filepath = os.path.join(host_dir, gs_utils.TIMESTAMP_COMPLETED_FILENAME)
    if os.path.isfile(ts_filepath):
      self.PushFileToDevice(ts_filepath, device_dir)

  def __init__(self, args, **kwargs):
    self._device = args['device']
    self._serial = args['serial']
    self._has_root = args['has_root'] == 'True'
    if self._serial == 'None':
      self._serial = android_utils.GetSerial(self._device)
    device_scratch_dir = shell_utils.Bash(
        '%s -s %s shell echo \$EXTERNAL_STORAGE' % (
            android_utils.PATH_TO_ADB, self._serial), 
        echo=True, shell=True).rstrip().split('\n')[-1]
    super(AndroidBuildStep, self).__init__(args=args, **kwargs)
    prefix = posixpath.join(device_scratch_dir, 'skiabot', 'skia_')
    self._device_dirs = DeviceDirs(perf_data_dir=prefix + 'perf',
                                   gm_dir=prefix + 'gm',
                                   gm_expected_dir=prefix + 'gm_expected',
                                   resource_dir=prefix + 'resources',
                                   skp_dir=prefix + 'skp',
                                   skp_perf_dir=prefix + 'skp_perf',
                                   skp_out_dir=prefix + 'skp_out')

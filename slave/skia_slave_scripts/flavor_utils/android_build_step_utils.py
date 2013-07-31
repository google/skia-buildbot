# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for Android build steps. """

from default_build_step_utils import DefaultBuildStepUtils, DeviceDirs
from utils import android_utils
from utils import gs_utils
from utils import shell_utils

import os
import posixpath


class AndroidBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    self._device = self._step.args['device']
    self._serial = self._step.args['serial'] if \
                             self._step.args['serial'] != 'None' else None
    self._has_root = self._step.args['has_root'] == 'True'

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
    if self.DevicePathExists(directory):
      raise Exception('Failed to remove %s' % directory)

  def _CreateDirectoryOnDevice(self, directory):
    """ Create a directory on the device. """
    android_utils.RunADB(self._serial, ['shell', 'mkdir', '-p', directory])

  def PushFileToDevice(self, src, dst):
    """ Overrides build_step.PushFileToDevice() """
    android_utils.RunADB(self._serial, ['push', src, dst])

  def DeviceListDir(self, directory):
    """ Overrides build_step.DeviceListDir() """
    return android_utils.ADBShell(self._serial, ['ls', directory],
                                  echo=False).split('\n')

  def DevicePathExists(self, path):
    """ Overrides build_step.DevicePathExists() """
    return 'FILE_EXISTS' in android_utils.ADBShell(
        self._serial,
        ['if', '[', '-e', path, '];', 'then', 'echo', 'FILE_EXISTS;', 'fi'])

  def DevicePathJoin(self, *args):
    """ Overrides build_step.DevicePathJoin() """
    return posixpath.sep.join(args)

  def CreateCleanDeviceDirectory(self, directory):
    self._RemoveDirectoryOnDevice(directory)
    self._CreateDirectoryOnDevice(directory)

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side.
    """
    self.CreateCleanDeviceDirectory(device_dir)
    file_list = os.listdir(host_dir)
    for f in file_list:
      if f == gs_utils.TIMESTAMP_COMPLETED_FILENAME:
        continue
      if os.path.isfile(os.path.join(host_dir, f)):
        self.PushFileToDevice(os.path.join(host_dir, f), device_dir)
    ts_filepath = os.path.join(host_dir, gs_utils.TIMESTAMP_COMPLETED_FILENAME)
    if os.path.isfile(ts_filepath):
      self.PushFileToDevice(ts_filepath, device_dir)

  def CopyDirectoryContentsToHost(self, device_dir, host_dir):
    """ Copy the contents of a device-side directory to a clean directory on the
    host side.
    """
    self.CreateCleanHostDirectory(host_dir)
    android_utils.RunADB(self._serial, ['pull', device_dir, host_dir])

  def Install(self):
    """ Install the Skia executables. """
    release_mode = self._step.configuration == 'Release'
    android_utils.Install(self._serial, release_mode,
                          install_launcher=self._has_root)

  def Compile(self, target):
    """ Compile the Skia executables. """
    os.environ['PATH'] = os.path.abspath(
        os.path.join(os.pardir, os.pardir, os.pardir, os.pardir, 'third_party',
                     'gsutil')) + os.pathsep + os.environ['PATH']
    os.environ['BOTO_CONFIG'] = os.path.abspath(os.path.join(
        os.pardir, os.pardir, os.pardir, os.pardir, 'site_config', '.boto'))
    os.environ['ANDROID_SDK_ROOT'] = self._step.args['android_sdk_root']
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    cmd = [os.path.join('platform_tools', 'android', 'bin', 'android_make'),
           target,
           '-d', self._step.args['device'],
           'BUILDTYPE=%s' % self._step.configuration,
           ]
    cmd.extend(self._step.default_make_flags)
    if os.name != 'nt':
      try:
        ccache = shell_utils.Bash(['which', 'ccache'], echo=False)
        if ccache:
          cmd.append('--use-ccache')
      except Exception:
        pass
    cmd.extend(self._step.make_flags)
    shell_utils.Bash(cmd)

  def PreRun(self):
    """ Preprocessing step to run before the BuildStep itself. """
    if self._serial:
      if self._has_root:
        android_utils.RunADB(self._serial, ['root'])
        android_utils.RunADB(self._serial, ['remount'])
        android_utils.SetCPUScalingMode(self._serial, 'performance')
        android_utils.ADBKill(self._serial, 'skia')
      else:
        android_utils.ADBKill(self._serial, 'com.skia', kill_app=True)

  def GetDeviceDirs(self):
    """ Set the directories which will be used by the BuildStep. """
    if self._serial:
      device_scratch_dir = shell_utils.Bash(
          '%s -s %s shell echo \$EXTERNAL_STORAGE' % (
              android_utils.PATH_TO_ADB, self._serial),
          echo=True, shell=True).rstrip().split('\n')[-1]
      prefix = posixpath.join(device_scratch_dir, 'skiabot', 'skia_')
      return DeviceDirs(perf_data_dir=prefix + 'perf',
                        gm_actual_dir=prefix + 'gm_actual',
                        gm_expected_dir=prefix + 'gm_expected',
                        resource_dir=prefix + 'resources',
                        skimage_in_dir=prefix + 'skimage_in',
                        skimage_expected_dir=prefix + 'skimage_expected',
                        skimage_out_dir=prefix + 'skimage_out',
                        skp_dir=prefix + 'skp',
                        skp_perf_dir=prefix + 'skp_perf',
                        skp_out_dir=prefix + 'skp_out',
                        tmp_dir=prefix + 'tmp_dir')

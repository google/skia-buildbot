# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""

from skia_master_scripts import factory as skia_factory

class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self._skia_cmd_obj.AddClean()

    self._skia_cmd_obj.AddRunCommand(
        command='../android/bin/android_make all -d nexus_s %s' % (
            self._make_flags),
        description='BuildAll')

    self.PushBinaryToDeviceAndRun(binary_name='tests', description='RunTests')
    self.PushBinaryToDeviceAndRun(binary_name='gm',
                                  arguments='--nopdf --noreplay',
                                  description='RunGM')
    self.PushBinaryToDeviceAndRun(binary_name='bench', description='RunBench')

    return self._factory

  def PushBinaryToDeviceAndRun(self, binary_name, arguments='',
                               description=None, timeout=None):
    """Adds a build step: push a binary file to the USB-connected Android
    device and run it.

    binary_name: which binary to run on the device
    arguments: additional arguments to pass to the binary when running it
    description: text description (e.g., 'RunTests')
    timeout: timeout in seconds, or None to use the default timeout

    The shell command (running on the buildbot slave) will exit with a nonzero
    return code if and only if the command running on the Android device
    exits with a nonzero return code... so a nonzero return code from the
    command running on the Android device will turn the buildbot red.
    """
    if not description:
      description = 'Run %s' % binary_name
    path_to_adb = self.TargetPathJoin('..', 'android', 'bin', 'linux', 'adb')
    command_list = [
        '%s root' % path_to_adb,
        '%s remount' % path_to_adb,
        '%s push out/%s/%s /system/bin/skia_%s' % (
            path_to_adb, self._configuration, binary_name, binary_name),
        '%s logcat -c' % path_to_adb,
        'STDOUT=$(%s shell "skia_%s %s && echo ADB_SHELL_SUCCESS")' % (
            path_to_adb, binary_name, arguments),
        'echo $STDOUT',
        '%s logcat -d' % path_to_adb,
        'echo $STDOUT | grep ADB_SHELL_SUCCESS',
        ]
    self._skia_cmd_obj.AddRunCommandList(
        command_list=command_list, description=description)

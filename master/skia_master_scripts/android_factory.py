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

    return self._factory

  def PushBinaryToDeviceAndRun(self, binary_name, description, timeout=None):
    """Adds a build step: push a binary file to the USB-connected Android
    device and run it.

    binary_name: which binary to run on the device
    description: text description (e.g., 'RunTests')
    timeout: timeout in seconds, or None to use the default timeout
    """
    path_to_adb = self.TargetPathJoin('..', 'android', 'bin', 'linux', 'adb')
    command_list = [
        '%s root' % path_to_adb,
        '%s remount' % path_to_adb,
        '%s push out/%s/%s /system/bin/%s' % (
            path_to_adb, self._configuration, binary_name, binary_name),
        '%s logcat -c' % path_to_adb,
        '%s shell %s' % (path_to_adb, binary_name),
        '%s logcat -d' % path_to_adb,
        ]
    self._skia_cmd_obj.AddRunCommandList(
        command_list=command_list, description=description)

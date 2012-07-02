# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""

from skia_master_scripts import factory as skia_factory

class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def Build(self, device, clobber=None):
    """Build and return the complete BuildFactory.

    device: string indicating which Android device type we are targeting
    clobber: boolean indicating whether we should clean before building
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self._skia_cmd_obj.AddClean()

    self._skia_cmd_obj.AddRunCommand(
        command='../android/bin/android_make all -d %s %s' % (device,
            self._make_flags),
        description='BuildAll')

    self.PushBinaryToDeviceAndRun(device=device, binary_name='tests',
                                  description='RunTests')
    self.PushBinaryToDeviceAndRun(device=device, binary_name='gm',
                                  arguments='--nopdf --noreplay',
                                  description='RunGM')
    self.PushBinaryToDeviceAndRun(device=device, binary_name='bench',
                                  description='RunBench')

    return self._factory

  def PushBinaryToDeviceAndRun(self, device, binary_name, arguments='',
                               description=None, timeout=None):
    """Adds a build step: push a binary file to the USB-connected Android
    device and run it.

    device: string indicating which Android device type we are targeting
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
    self._skia_cmd_obj.AddRunAndroid(device=device, binary_name=binary_name,
                                     args=arguments, description=description)

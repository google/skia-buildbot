# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""

from skia_master_scripts import factory as skia_factory

class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def Compile(self):
    """Compile step.  Build everything. """
    environment = 'ANDROID_SDK_ROOT=/home/chrome-bot/android-sdk-linux'
    self._skia_cmd_obj.AddRunCommand(
        command='%s ../android/bin/android_make all -d %s %s' % (
                    environment, self._device, self._make_flags),
        description='BuildAll')
    # Install the app onto the device, so that it can be used in later steps.
    self._skia_cmd_obj.AddInstallAndroid(device=self._device)

  def RunTests(self):
    """ Run the unit tests. """
    self.PushBinaryToDeviceAndRun(device=self._device, binary_name='tests',
                                  description='RunTests')

  def RunGM(self, path_to_gm, gm_actual_dir):
    """ Run the "GM" tool, saving the images to disk. """
    self._skia_cmd_obj.AddAndroidRunGM(device=self._device,
                                       arguments='--nopdf --noreplay')

  def CompareGMs(self, gm_actual_dir):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    command_list = [
        'make clean',
        'make tools %s' % self._make_flags,
        ]
    self._skia_cmd_obj.AddRunCommandList(
        command_list=command_list, description='BuildSkDiff')
    skia_factory.SkiaFactory.CompareGMs(self, gm_actual_dir)

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    # TODO(borenet): Actually pipe the output somewhere, or update the master to
    # capture the output.
    self.PushBinaryToDeviceAndRun(device=self._device, binary_name='bench',
                                  description='RunBench')

  def Build(self, device, clobber=None):
    """Build and return the complete BuildFactory.

    device: string indicating which Android device type we are targeting
    clobber: boolean indicating whether we should clean before building
    """
    self._device = device
    return super(AndroidFactory, self).Build(clobber)

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

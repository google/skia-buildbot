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
    args = ['--target', 'all']
    self.AddSlaveScript(script='android_compile.py', args=args,
                        description='BuildAll')
    # Install the app onto the device, so that it can be used in later steps.
    self.AddSlaveScript(script='android_install_apk.py',
                        description='InstallAPK')

  def RunTests(self):
    """ Run the unit tests. """
    self.AddSlaveScript(script='android_run_tests.py', description='RunTests')

  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddSlaveScript(script='android_run_gm.py', description='GenerateGMs')

  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddSlaveScript(script='android_render_pictures.py',
                        description='RenderPictures')

  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    self.AddSlaveScript(script='clean.py', description='Clean')
    self.Make('tools', 'BuildSkDiff')
    super(AndroidFactory, self).CompareGMs()

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddSlaveScript(script='android_run_bench.py', description='RunBench')

  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddSlaveScript(script='android_bench_pictures.py',
                        description='BenchPictures')

  def Build(self, device, serial=None, clobber=None):
    """Build and return the complete BuildFactory.

    device: string indicating which Android device type we are targeting
    serial: optional, string indicating the serial number of a specific Android
            device to target.  If this is None, the builder will use the first
            attached device whose description matches the device parameter.
    clobber: boolean indicating whether we should clean before building
    """
    self._device = device
    self._serial = serial or 'None'
    self._common_args += ['--device', self._device,
                          '--serial', self._serial]
    return super(AndroidFactory, self).Build(clobber)

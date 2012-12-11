# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""

from skia_master_scripts import factory as skia_factory
from buildbot.process.properties import WithProperties

class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def __init__(self, device, other_subdirs=None, **kwargs):
    """ Instantiates an AndroidFactory with properties and build steps specific
    to Android devices.

    device: string indicating which Android device type we are targeting
    """
    if not other_subdirs:
      other_subdirs = []
    skia_factory.SkiaFactory.__init__(self, other_subdirs, **kwargs)
    self._device = device
    self._common_args += ['--device', self._device,
                          '--serial', WithProperties('%(serial:-None)s')]

  def Compile(self, clobber=None):
    """Compile step. Build everything.

    clobber: optional boolean which tells us whether to 'clean' before building.
    """
    self.AddSlaveScript(script='clean.py', description='Clean',
                        is_rebaseline_step=True)

    args = ['--target', 'all']
    self.AddSlaveScript(script='android_compile.py', args=args,
                        description='BuildAll', halt_on_failure=True,
                        is_rebaseline_step=True)
    # Install the app onto the device, so that it can be used in later steps.
    self.AddSlaveScript(script='android_install_apk.py',
                        description='InstallAPK', halt_on_failure=True,
                        is_rebaseline_step=True)

  def RunTests(self):
    """ Run the unit tests. """
    self.AddSlaveScript(script='android_run_tests.py', description='RunTests')

  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddSlaveScript(script='android_run_gm.py', description='GenerateGMs',
                        is_rebaseline_step=True)

  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddSlaveScript(script='android_render_pictures.py',
                        description='RenderPictures')

  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    self.AddSlaveScript(script='clean.py', description='Clean',
                        is_rebaseline_step=True)
    self.Make('tools', 'BuildSkDiff', is_rebaseline_step=True)
    skia_factory.SkiaFactory.CompareGMs(self)

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddSlaveScript(script='android_run_bench.py', description='RunBench')

  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddSlaveScript(script='android_bench_pictures.py',
                        description='BenchPictures')

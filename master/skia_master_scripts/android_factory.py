# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""


from buildbot.process.properties import WithProperties
from skia_master_scripts import factory as skia_factory


class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def __init__(self, device, other_subdirs=None, **kwargs):
    """ Instantiates an AndroidFactory with properties and build steps specific
    to Android devices.

    device: string indicating which Android device type we are targeting
    """
    if not other_subdirs:
      other_subdirs = []
    subdirs_to_checkout = set(other_subdirs)
    subdirs_to_checkout.add('android')
    skia_factory.SkiaFactory.__init__(self, other_subdirs=subdirs_to_checkout,
                                      bench_pictures_cfg=device,
                                      deps_target_os='android',
                                      flavor='android', **kwargs)
    self._device = device
    self._common_args += ['--device', self._device,
                          '--serial', WithProperties('%(serial:-None)s'),
                          '--has_root', WithProperties('%(has_root:-True)s'),
                          '--android_sdk_root',
                              WithProperties('%(android_sdk_root)s')]

  def Compile(self, clobber=None, build_in_one_step=True):
    """ Compile step. Build everything.

    clobber: optional boolean; whether to 'clean' before building. Ignored on
        Android.
    build_in_one_step: optional boolean; whether to build in one step or build
        each target separately. Ignored on Android.
    """
    self.AddSlaveScript(script='clean.py', description='Clean',
                        is_rebaseline_step=True)

    # On Android, we build all targets at once.  This is because the Android app
    # is always built with any target, and the build system is not smart enough
    # to know when the set of packaged libraries has changed, which causes the
    # app not to contain the full set of Skia libraries.
    args = ['--target', 'all']
    self.AddSlaveScript(script='android_compile.py', args=args,
                        description='BuildAll', halt_on_failure=True,
                        is_rebaseline_step=True)

  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    self.AddSlaveScript(script='clean.py', description='Clean',
                        is_rebaseline_step=True)
    self.Make('tools', 'BuildSkDiff', is_rebaseline_step=True)
    skia_factory.SkiaFactory.CompareGMs(self)

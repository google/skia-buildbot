# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""


from buildbot.process.properties import WithProperties
from skia_master_scripts import factory as skia_factory


class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def __init__(self, device, **kwargs):
    """ Instantiates an AndroidFactory with properties and build steps specific
    to Android devices.

    device: string indicating which Android device type we are targeting
    """
    skia_factory.SkiaFactory.__init__(self, bench_pictures_cfg=device,
                                      deps_target_os='android',
                                      flavor='android',
                                      build_targets=['all'],
                                      **kwargs)
    self._device = device
    self._common_args += ['--device', self._device,
                          '--serial', WithProperties('%(serial:-None)s'),
                          '--has_root', WithProperties('%(has_root:-True)s'),
                          '--android_sdk_root',
                              WithProperties('%(android_sdk_root)s')]
    self._default_clobber = True


  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    # We have bypass the Android-flavored compile in order to build SkDiff for
    # the host.
    self.AddSlaveScript(script='compile.py',
                        description='BuildSkDiff',
                        is_rebaseline_step=True,
                        args=['--target', 'tools',
                              '--gyp_defines',
                              ' '.join('%s=%s' % (k, v)
                                       for k, v in self._gyp_defines.items())])
    skia_factory.SkiaFactory.CompareGMs(self)

# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from factory import SkiaFactory, CONFIG_RELEASE

import builder_name_schema


class Moz2DCanaryFactory(SkiaFactory):
  """ Subclass of Factory which builds Moz2D. """

  def __init__(self, **kwargs):
    """ Instantiates a Moz2DFactory with properties and build steps specific to
    Moz2D builds. """
    SkiaFactory.__init__(self, flavor='moz2d_canary',
                         build_targets=['skia_lib', 'moz2d'],
                         other_subdirs=['https://github.com/gw280/moz2d.git'],
                         configuration=CONFIG_RELEASE,
                         **kwargs)
    self._default_clobber = True

  def Update(self):
    """ Update the Skia code on the build slave. """
    args = ['--gclient_solutions', '"%s"' % self._gclient_solutions]
    self.AddSlaveScript(
        script=self.TargetPath.join('..', '..', '..', '..', '..', 'slave',
                                   'skia_slave_scripts',
                                   '%s_update.py' % self._flavor),
        description='Update',
        args=args,
        timeout=None,
        halt_on_failure=True,
        is_upload_step=False,
        is_rebaseline_step=True,
        get_props_from_stdout={'got_revision':'Skia updated to revision (\d+)',
                               'moz2d_revision': 'Moz2D updated to (\w+)'},
        workdir='build')

  def Build(self, **kwargs):
    # Spoof the role as a compile builder so that this factory only runs the
    # update and compile steps.
    return SkiaFactory.Build(self,
                             role=builder_name_schema.BUILDER_ROLE_BUILD,
                             **kwargs)
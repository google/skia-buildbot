# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from buildbot.process.properties import WithProperties
from factory import SkiaFactory


class NaClFactory(SkiaFactory):
  """ Subclass of Factory which runs in Native Client. """

  def __init__(self, other_subdirs=None, **kwargs):
    """ Instantiates a NaClFactory with properties and build steps specific to
    Native Client builds. """
    if not other_subdirs:
      other_subdirs = []
    subdirs_to_checkout = set(other_subdirs)
    subdirs_to_checkout.add('nacl')
    SkiaFactory.__init__(self, other_subdirs=subdirs_to_checkout, flavor='nacl',
                         **kwargs)
    self._common_args += ['--nacl_sdk_root',
                              WithProperties('%(nacl_sdk_root)s')]

  def Make(self, target, description, is_rebaseline_step=False):
    """ Build a single target.

    target: string; the target to build.
    description: string; description of this BuildStep.
    is_rebaseline_step: optional boolean; whether or not this step is required
        for rebaseline-only builds.
    """
    args = ['--target', target]
    self.AddSlaveScript(script='nacl_compile.py', args=args,
                        description=description, halt_on_failure=True,
                        is_rebaseline_step=is_rebaseline_step)

  def Compile(self, clobber=None, build_in_one_step=True):
    """ Compile step. Build everything that is currently supported on NaCl.

    clobber: optional boolean; whether to 'clean' before building.
    build_in_one_step: optional boolean; whether to build in one step or build
        each target separately.
    """
    if clobber is None:
      clobber = self._default_clobber

    # Trybots should always clean.
    if clobber or self._do_patch_step:
      self.AddSlaveScript(script='clean.py', description='Clean')

    self.Make('skia_base_libs', 'BuildSkiaBaseLibs')
    self.Make('tests', 'BuildTests')
    self.Make('debugger', 'BuildDebugger')


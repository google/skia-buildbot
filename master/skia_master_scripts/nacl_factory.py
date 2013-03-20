# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


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

  def Make(self, target, description, is_rebaseline_step=False):
    """ Build a single target."""
    args = ['--target', target]
    self.AddSlaveScript(script='nacl_compile.py', args=args,
                        description=description, halt_on_failure=True,
                        is_rebaseline_step=is_rebaseline_step)

  def Compile(self, clobber=None):
    """Compile step. Build everything that is currently supported on NaCl.

    clobber: optional boolean which tells us whether to 'clean' before building.
    """
    if clobber is None:
      clobber = self._default_clobber

    # Trybots should always clean.
    if clobber or self._do_patch_step:
      self.AddSlaveScript(script='clean.py', description='Clean')

    self.Make('skia_base_libs', 'BuildSkiaBaseLibs')
    self.Make('tests', 'BuildTests')
    self.Make('debugger', 'BuildDebugger')

  def Build(self, clobber=None):
    self.UpdateSteps()
    self.Compile(clobber)
    return self
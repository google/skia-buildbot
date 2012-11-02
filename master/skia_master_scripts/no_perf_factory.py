# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.



from android_factory import AndroidFactory
from factory import SkiaFactory

class NoPerfFactory(SkiaFactory):
  """Subclass of Factory which does not run benchmarking steps. Designed to
  complement PerfOnlyFactory. """
  def Build(self, clobber=None):
    if self._perf_output_basedir:
      raise ValueError('NoPerfFactory does not run benchmarking steps and '
                       'therefore perf_output_basedir should not be defined.')
    self.UpdateSteps()
    self.Compile(clobber)
    self.NonPerfSteps()
    return self

class AndroidNoPerfFactory(AndroidFactory, NoPerfFactory):
  """ Android-specific subclass of NoPerfFactory.  Inherits __init__() from
  AndroidFactory and Build() from NoPerfFactory. """
  def __init__(self, **kwargs):
    AndroidFactory.__init__(self, **kwargs)

  def Build(self, **kwargs):
    return NoPerfFactory.Build(self, **kwargs)
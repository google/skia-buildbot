# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from android_factory import AndroidFactory
from factory import SkiaFactory, CONFIG_RELEASE

class PerfOnlyFactory(SkiaFactory):
  """Subclass of Factory which only runs benchmarking steps. Designed to
  complement NoPerfFactory. """

  def Compile(self, clobber):
    """Compile step. Build everything.

    clobber: optional boolean which tells us whether to 'clean' before building.
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', description='Clean')
    self.Make('bench','BuildBench')
    self.Make('tools','BuildTools')

  def Build(self, clobber=None):
    if not self._perf_output_basedir:
      raise ValueError(
          'PerfOnlyFactory requires perf_output_basedir to be defined.')
    if self._configuration != CONFIG_RELEASE:
      raise ValueError('PerfOnlyFactory should run in %s configuration.' %
                           CONFIG_RELEASE)
    self.UpdateSteps()
    self.Compile(clobber)
    self.PerfSteps()
    return self

class AndroidPerfOnlyFactory(AndroidFactory, PerfOnlyFactory):
  """ Android-specific subclass of PerfOnlyFactory.  Inherits __init__() from
  AndroidFactory and Build() from PerfOnlyFactory. """
  pass
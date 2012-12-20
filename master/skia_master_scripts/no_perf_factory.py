# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from android_factory import AndroidFactory
from chromeos_factory import ChromeOSFactory
from factory import SkiaFactory
from ios_factory import iOSFactory


class NoPerfFactory(SkiaFactory):
  """Subclass of Factory which does not run benchmarking steps. Designed to
  complement PerfOnlyFactory. """
  def Build(self, clobber=None):
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


class ChromeOSNoPerfFactory(ChromeOSFactory, NoPerfFactory):
  """ ChromeOS-specific subclass of NoPerfFactory.  Inherits __init__() from
  ChromeOSFactory and Build() from NoPerfFactory. """
  def __init__(self, **kwargs):
    ChromeOSFactory.__init__(self, **kwargs)

  def Build(self, **kwargs):
    return NoPerfFactory.Build(self, **kwargs)


class iOSNoPerfFactory(iOSFactory, NoPerfFactory):
  """ iOS-specific subclass of NoPerfFactory.  Inherits __init__() from
  iOSFactory and Build() from NoPerfFactory. """
  def __init__(self, **kwargs):
    iOSFactory.__init__(self, **kwargs)

  def Build(self, **kwargs):
    # TODO: Inheriting Build() from iOSFactory until all build steps are
    # supported.
    return iOSFactory.Build(self, **kwargs)
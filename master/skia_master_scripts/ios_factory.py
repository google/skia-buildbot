# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from factory import SkiaFactory

class iOSFactory(SkiaFactory):
  """ Subclass of Factory which runs on iOS. """
  def Build(self, clobber=None):
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', description='Clean')
    self.Compile()
    return self._factory
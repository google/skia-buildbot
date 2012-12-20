# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from factory import SkiaFactory


class iOSFactory(SkiaFactory):
  """ Subclass of Factory which runs on iOS. """
  def Build(self, clobber=None):
    self.UpdateSteps()
    self.Compile(clobber)
    return self
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from skia_master_scripts import factory as skia_factory

class XsanFactory(skia_factory.SkiaFactory):
  def __init__(self, sanitizer, **kwargs):
    """ Sets up a factory for builds using one of Clang's sanitizer modes.

    sanitizer: name of the sanitizer to use (e.g. address, thread, undefined)
    """
    skia_factory.SkiaFactory.__init__(self, flavor='xsan', **kwargs)
    self._common_args += ['--sanitizer', sanitizer]


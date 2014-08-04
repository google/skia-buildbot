# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from buildbot.process.properties import WithProperties
from factory import SkiaFactory


class NaClFactory(SkiaFactory):
  """ Subclass of Factory which runs in Native Client. """

  def __init__(self, **kwargs):
    """ Instantiates a NaClFactory with properties and build steps specific to
    Native Client builds. """
    SkiaFactory.__init__(self, flavor='nacl',
                         build_targets=['skia_lib', 'debugger'],
                         **kwargs)
    self._common_args += ['--nacl_sdk_root',
                              WithProperties('%(nacl_sdk_root)s')]

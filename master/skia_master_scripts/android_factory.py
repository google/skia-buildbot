# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for Android buildbots.

Overrides SkiaFactory with any Android-specific steps."""

from skia_master_scripts import factory as skia_factory

class AndroidFactory(skia_factory.SkiaFactory):
  """Overrides for Android builds."""

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self._skia_cmd_obj.AddClean()

    self._skia_cmd_obj.AddRun(
        run_command='../android/bin/android_make all -d xoom %s' % (
            self._make_flags),
        description='BuildAll')

    return self._factory

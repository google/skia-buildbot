# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from factory import SkiaFactory

import builder_name_schema


class AndroidRollFactory(SkiaFactory):
  """Factory which rolls Skia revisions into Android."""

  def Build(self, role=builder_name_schema.BUILDER_ROLE_HOUSEKEEPER,
            clobber=None, **kwargs):
    """Build and return the complete BuildFactory.

    role: string; type of builder.
    clobber: boolean indicating whether we should clean before building
    """
    if role != builder_name_schema.BUILDER_ROLE_HOUSEKEEPER:
      raise Exception('Housekeeping builders must have role "%s"' %
                      builder_name_schema.BUILDER_ROLE_HOUSEKEEPER)

    self.UpdateSteps()
    self.AddSlaveScript(script='sync_android.py', description='SyncAndroid')
    self.AddSlaveScript(script='merge_into_android.py', description='Merge')
    self.Validate()
    return self


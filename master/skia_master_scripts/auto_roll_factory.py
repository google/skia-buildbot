# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from canary_factory import CanaryFactory

import builder_name_schema


class AutoRollFactory(CanaryFactory):
  """Factory which runs the Blink AutoRoll bot for Skia."""

  def __init__(self, **kwargs):
    """Instantiates an AutoRollFactory"""
    CanaryFactory.__init__(self,
                           flavor='chrome',
                           path_to_skia=['third_party', 'skia'],
                           **kwargs)

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
    self.AddSlaveScript(script='do_auto_roll.py', description='AutoRoll')
    self.Validate()
    return self


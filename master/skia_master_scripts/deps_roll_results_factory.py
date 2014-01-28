# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""BuildFactory for a builder which reads the results of a DEPS roll builder."""


from skia_master_scripts import factory

import builder_name_schema


class DepsRollResultsFactory(factory.SkiaFactory):
  """Reads the results of a DEPS roll builder."""

  def __init__(self, deps_roll_builder, **kwargs):
    """Initialize the DepsRollFactory."""
    factory.SkiaFactory.__init__(self, **kwargs)
    self._deps_roll_builder = deps_roll_builder

  def Build(self, role=builder_name_schema.BUILDER_ROLE_HOUSEKEEPER,
            clobber=None):
    """Build and return the complete BuildFactory.

    role: string; type of builder.
    clobber: boolean indicating whether we should clean before building
    """
    if role != builder_name_schema.BUILDER_ROLE_HOUSEKEEPER:
      raise Exception('Housekeeping builders must have role "%s"' %
                      builder_name_schema.BUILDER_ROLE_HOUSEKEEPER)
    self.UpdateSteps()
    self.AddSlaveScript(script='collect_deps_roll_trybot_results.py',
                        description='CollectDEPSRollTrybotResults',
                        args=['--upstream_bot', self._deps_roll_builder])
    self.Validate()
    return self
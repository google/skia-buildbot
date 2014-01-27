# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""BuildFactory for a builder which creates DEPS roll CLs."""


from skia_master_scripts import canary_factory

import builder_name_schema


class DepsRollFactory(canary_factory.CanaryFactory):
  """Overrides for HouseKeeping periodic builds."""

  def __init__(self, **kwargs):
    """Initialize the DepsRollFactory."""
    canary_factory.CanaryFactory.__init__(self,
                                          path_to_skia=['third_party', 'skia'],
                                          flavor='chrome',
                                          build_subdir='src',
                                          **kwargs)

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
    self.AddSlaveScript(script='do_deps_roll.py',
                        description='DEPSRoll',
                        get_props_from_stdout={
                            'deps_roll_issue': 'Deps roll Issue number: (\d+) ',
                            'control_issue': 'Control Issue number: (\d+) ',
                        })
    self.Validate()
    return self
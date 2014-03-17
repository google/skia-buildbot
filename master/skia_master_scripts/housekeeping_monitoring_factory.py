# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Factory for a builder which monitors the buildbots."""


from skia_master_scripts import factory as skia_factory
import builder_name_schema


class HouseKeepingMonitoringFactory(skia_factory.SkiaFactory):

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

    self.AddSlaveScript(script='update_all_slave_hosts.py',
                        description='UpdateSlaveHosts')
    self.AddSlaveScript(script='check_buildslave_host_disk_usage.py',
                        description='CheckBuildslaveHostDiskUsage')
    self.Validate()
    return self
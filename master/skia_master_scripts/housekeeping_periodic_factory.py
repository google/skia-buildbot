# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with Periodic HouseKeeping steps."""

import builder_name_schema

from config_private import SKIA_PUBLIC_MASTER_INTERNAL_FQDN
from skia_master_scripts import factory as skia_factory


# TODO: The HouseKeepingPeriodicFactory uses shell scripts, so it would break on
# Windows. For now, we reply on the fact that the housekeeping bot always runs
# on a Linux machine.
class HouseKeepingPeriodicFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping periodic builds."""

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
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', description='Clean')

    # pylint: disable=W0212
    clang_static_analyzer_script_path = self.TargetPath.join(
        self._skia_cmd_obj._local_slave_script_dir,
        'run-clang-static-analyzer.sh')
    self._skia_cmd_obj.AddRunCommand(
        command=clang_static_analyzer_script_path,
        description='RunClangStaticAnalyzer')

    self.AddSlaveScript(script='check_gs_timestamps.py',
                        description='CheckGoogleStorageTimestamps')

    if not self._do_patch_step:  # Do not run the checkers if it is a try job.
      # pylint: disable=W0212
      disk_usage_script_path = self.TargetPath.join(
          self._skia_cmd_obj._local_slave_script_dir,
          'check_compute_engine_disk_usage.sh')
      self._skia_cmd_obj.AddRunCommand(
          command=('SKIA_COMPUTE_ENGINE_HOSTNAME=%s PERSISTENT_DISK_NAME='
                   '/home/default/skia-repo %s'
                   % (SKIA_PUBLIC_MASTER_INTERNAL_FQDN,
                      disk_usage_script_path)),
          description='CheckMasterDiskUsage')

    self.Validate()
    return self

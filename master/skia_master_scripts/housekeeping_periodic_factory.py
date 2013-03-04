# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with Periodic HouseKeeping steps."""

import tempfile

from buildbot.process.properties import WithProperties
from config_private import SKIA_PUBLIC_MASTER, SKIA_SVN_BASEURL
from skia_master_scripts import factory as skia_factory


# TODO: The HouseKeepingPeriodicFactory uses shell scripts, so it would break on
# Windows. For now, we reply on the fact that the housekeeping bot always runs
# on a Linux machine.
class HouseKeepingPeriodicFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping periodic builds."""

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    self.UpdateSteps()
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', description='Clean')

    if not self._do_patch_step:  # Do not run the sanitizer if it is a try job.
      sanitize_script_path = self.TargetPathJoin('tools',
                                                 'sanitize_source_files.py')
      skia_trunk_svn_baseurl = '%s/%s' % (
          SKIA_SVN_BASEURL.replace('http', 'https'), 'trunk')
      # Run the sanitization script.
      self._skia_cmd_obj.AddRunCommand(
          command='python %s' % sanitize_script_path,
          description='RunSanitization')
      if self._do_upload_results:
        merge_dir_path = self.TargetPathJoin(tempfile.gettempdir(),
                                             'sanitize-merge')
        # Cleanup the previous (if any) sanitize merge dir.
        self._skia_cmd_obj.AddRunCommand(
          command='rm -rf %s' % merge_dir_path, description='Cleanup')
        # Upload sanitized files.
        self._skia_cmd_obj.AddMergeIntoSvn(
            source_dir_path='.',
            dest_svn_url=skia_trunk_svn_baseurl,
            merge_dir_path=merge_dir_path,
            svn_username_file=self._skia_svn_username_file,
            svn_password_file=self._skia_svn_password_file,
            commit_message=WithProperties(
                'Sanitizing source files in %s' % self._builder_name),
            description='UploadSanitizedFiles')

    # pylint: disable=W0212
    clang_static_analyzer_script_path = self.TargetPathJoin(
        self._skia_cmd_obj._local_slave_script_dir,
        'run-clang-static-analyzer.sh')
    self._skia_cmd_obj.AddRunCommand(
        command=clang_static_analyzer_script_path,
        description='RunClangStaticAnalyzer')

    self.AddSlaveScript(script='check_gs_timestamps.py',
                        description='CheckGoogleStorageTimestamps')

    if not self._do_patch_step:  # Do not run the checkers if it is a try job.
      # pylint: disable=W0212
      disk_usage_script_path = self.TargetPathJoin(
          self._skia_cmd_obj._local_slave_script_dir,
          'check_compute_engine_disk_usage.sh')
      self._skia_cmd_obj.AddRunCommand(
          command=('SKIA_COMPUTE_ENGINE_HOSTNAME=%s PERSISTENT_DISK_NAME='
                   '/home/default/skia-master %s'
                   % (SKIA_PUBLIC_MASTER, disk_usage_script_path)),
          description='CheckMasterDiskUsage')
      self._skia_cmd_obj.AddRunCommand(
          command=(WithProperties('SKIA_COMPUTE_ENGINE_HOSTNAME=%(slavename)s '
                                  'PERSISTENT_DISK_NAME='
                                  '/home/default/skia-slave ' + \
                                  disk_usage_script_path)),
          description='CheckHousekeepingSlaveDiskUsage')

    return self


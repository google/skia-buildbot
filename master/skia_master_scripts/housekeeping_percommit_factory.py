# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with Per-commit HouseKeeping steps."""

import os
import tempfile

from buildbot.process.properties import WithProperties
from config_private import AUTOGEN_SVN_BASEURL
from skia_master_scripts import factory as skia_factory


# TODO: The HouseKeepingPerCommitFactory uses shell scripts, so it would break
# on Windows. For now, we reply on the fact that the housekeeping bot always
# runs on a Linux machine.
class HouseKeepingPerCommitFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping per-commit builds."""
  def __init__(self, **kwargs):
    skia_factory.SkiaFactory.__init__(self, build_targets=['tools', 'gm'],
                                      **kwargs)

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    self.UpdateSteps()
    self.Compile(clobber)

    # Build tools and run their unittests.
    self._skia_cmd_obj.AddRunCommand(
        command=self.TargetPath.join('tools', 'tests', 'run.sh'),
        description='RunToolSelfTests')

    # Build GM and run its unittests.
    self._skia_cmd_obj.AddRunCommand(
        command=self.TargetPath.join('gm', 'tests', 'run.sh'),
        description='RunGmSelfTests')

    # Check for static initializers.
    self.AddSlaveScript(script='detect_static_initializers.py',
                        description='DetectStaticInitializers')

    if not self._do_patch_step:  # Do not run Pydoc & Doxygen steps if try job.
      # Generate and upload Buildbot Pydoc documentation.
      buildbot_pydoc_actual_svn_baseurl = '%s/%s' % (AUTOGEN_SVN_BASEURL,
                                                     'buildbot-docs')
      # pylint: disable=W0212
      update_buildbot_pydoc_path = self.TargetPath.join(
          self._skia_cmd_obj._local_slave_script_dir,
          'update-buildbot-pydoc.sh')
      buildbot_pydoc_working_dir = self.TargetPath.join(
          tempfile.gettempdir(), 'buildbot-docs')
      # Cleanup the previous (if any) buildbot pydoc working dir.
      self._skia_cmd_obj.AddRunCommand(
          command='rm -rf %s' % buildbot_pydoc_working_dir,
          description='CleanupBuildbotPydoc')
      # Generate Buildbot Pydoc documentation.
      self._skia_cmd_obj.AddRunCommand(
          command='BUILDBOT_PYDOC_TEMPDIR=%s bash %s' % (
              buildbot_pydoc_working_dir, update_buildbot_pydoc_path),
          description='UpdateBuildbotPydoc')
      if self._do_upload_results:
        # Upload Buildbot Pydoc.
        self._skia_cmd_obj.AddMergeIntoSvn(
            source_dir_path=self.TargetPath.join(
                buildbot_pydoc_working_dir, 'buildbot-docs'),
            dest_svn_url=buildbot_pydoc_actual_svn_baseurl,
            merge_dir_path=os.path.join(buildbot_pydoc_working_dir, 'merge'),
            svn_username_file=self._autogen_svn_username_file,
            svn_password_file=self._autogen_svn_password_file,
            commit_message=WithProperties(
                'UploadBuildbotPydoc of r%%(%s:-)s on %s' % (
                    'revision', self._builder_name)),
            description='UploadBuildbotPydoc')

      # Generate and upload Doxygen documentation.
      doxygen_actual_svn_baseurl = '%s/%s' % (AUTOGEN_SVN_BASEURL, 'docs')
      update_doxygen_path = self.TargetPath.join('tools', 'update-doxygen.sh')
      doxygen_working_dir = self.TargetPath.join(
          tempfile.gettempdir(), 'doxygen')
      # Cleanup the previous (if any) doxygen working dir.
      self._skia_cmd_obj.AddRunCommand(
          command='rm -rf %s' % doxygen_working_dir,
          description='CleanupDoxygen')
      # Generate Doxygen documentation.
      self._skia_cmd_obj.AddRunCommand(
          command='DOXYGEN_TEMPDIR=%s DOXYGEN_COMMIT=false bash %s' % (
              doxygen_working_dir, update_doxygen_path),
          description='UpdateDoxygen')
      if self._do_upload_results:
        # Upload Doxygen.
        self._skia_cmd_obj.AddMergeIntoSvn(
            source_dir_path=self.TargetPath.join(
                doxygen_working_dir, 'docs'),
            dest_svn_url=doxygen_actual_svn_baseurl,
            merge_dir_path=os.path.join(doxygen_working_dir, 'merge'),
            svn_username_file=self._autogen_svn_username_file,
            svn_password_file=self._autogen_svn_password_file,
            commit_message=WithProperties(
                'UploadDoxygen of r%%(%s:-)s on %s' % (
                    'revision', self._builder_name)),
            description='UploadDoxygen')

    self._skia_cmd_obj.AddRunCommand(
        command='python run_unittests', description='BuildbotSelfTests',
        workdir=self.TargetPath.join(os.pardir, os.pardir, os.pardir,
                                     os.pardir))

    self.AddSlaveScript(script='check_compile_times.py',
                        description='CheckCompileTimes')
    self.Validate()
    return self

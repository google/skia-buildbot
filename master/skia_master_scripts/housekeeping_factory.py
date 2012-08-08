# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with any Android-specific steps."""

import os
import tempfile
import shutil

from buildbot.process.properties import WithProperties
from skia_master_scripts import factory as skia_factory


class HouseKeepingFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping builds."""

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', args=[], description='Clean')

    # Build tools and run their unittests.
    # TODO: this runs a shell script, so would break on Windows. For now, we
    # rely on the fact that the housekeeping bot always runs on a Linux machine.
    self._skia_cmd_obj.AddRunCommand(
        command='make tools %s' % self._make_flags,
        description='BuildTools')
    self._skia_cmd_obj.AddRunCommand(
        command=self.TargetPathJoin('tools', 'tests', 'run.sh'),
        description='RunToolSelfTests')

    # Generate and upload Doxygen documentation.
    # TODO: this runs a shell script, so would break on Windows. For now, we
    # rely on the fact that the housekeeping bot always runs on a Linux machine.
    doxygen_actual_svn_baseurl = '%s/%s' % (
        skia_factory.AUTOGEN_SVN_BASEURL, 'docs')
    update_doxygen_path = self.TargetPathJoin('tools', 'update-doxygen.sh')
    # TODO: the following line creates a temporary directory on the MASTER,
    # and then uses its path on the SLAVE.  We should fix that.
    doxygen_working_dir = tempfile.mkdtemp()
    try:
      self._skia_cmd_obj.AddRunCommand(
          command='DOXYGEN_TEMPDIR=%s DOXYGEN_COMMIT=false bash %s' % (
              doxygen_working_dir, update_doxygen_path),
          description='UpdateDoxygen')
      if self._do_upload_results:
        # Upload Doxygen
        self._skia_cmd_obj.AddMergeIntoSvn(
            source_dir_path=self.TargetPathJoin(
                doxygen_working_dir, 'docs'),
            dest_svn_url=doxygen_actual_svn_baseurl,
            merge_dir_path=os.path.join(doxygen_working_dir, 'merge'),
            svn_username_file=self._autogen_svn_username_file,
            svn_password_file=self._autogen_svn_password_file,
            commit_message=WithProperties(
                'UploadDoxygen of r%%(%s:-)s on %s' % (
                    'revision', self._builder_name)),
            description='UploadDoxygen')
    finally:
      shutil.rmtree(doxygen_working_dir)

    return self._factory

# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with any Android-specific steps."""

import tempfile
import shutil

from buildbot.process.properties import WithProperties
from skia_master_scripts import factory as skia_factory


class HouseKeepingFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping builds."""

  def Build(self):
    """Build and return the complete BuildFactory."""

    # Figure out where we are going to store updated Doxygen files.
    doxygen_actual_svn_baseurl = '%s/%s' % (
        skia_factory.AUTOGEN_SVN_BASEURL, 'docs')
    update_doxygen_path = self.TargetPathJoin('tools', 'update-doxygen.sh')
    doxygen_working_dir = tempfile.mkdtemp()

    try:
      self._skia_cmd_obj.AddRunCommand(
          command='DOXYGEN_TEMPDIR=%s DOXYGEN_COMMIT=false bash %s' % (
              doxygen_working_dir, update_doxygen_path),
          description='Update Doxygen')
      if self._do_upload_results:
        # Upload Doxygen
        self._skia_cmd_obj.AddMergeIntoSvn(
            source_dir_path=self.TargetPathJoin(
                doxygen_working_dir, 'docs'),
            dest_svn_url=doxygen_actual_svn_baseurl,
            svn_username_file=self._autogen_svn_username_file,
            svn_password_file=self._autogen_svn_password_file,
            commit_message=WithProperties(
                'UploadDoxygen of r%%(%s:-)s on %s' % (
                    'revision', self._builder_name)),
            description='Upload Doxygen')
    finally:
      shutil.rmtree(doxygen_working_dir)

    return self._factory


#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Uploads the results of render_pictures.py.

TODO(epoger): In the midst of re-implementing using checksums;
see https://code.google.com/p/skia/issues/detail?id=1942

TODO(epoger): Rename as upload_rendered_skps.py (and rename class as
UploadRenderedSKPs), and add a separate compare_rendered_skps.py .

"""

import sys

from build_step import BuildStep
import upload_gm_results


class CompareAndUploadWebpageGMs(upload_gm_results.UploadGMResults):

  def _Run(self):
    # TODO(epoger): Temporary hack to make this work until a master restart
    # causes these arguments to be passed to this slave script.
    if 'autogen_svn_username_file' not in self._args:
      self._args['autogen_svn_username_file'] = '.autogen_svn_username'
    if 'autogen_svn_password_file' not in self._args:
      self._args['autogen_svn_password_file'] = '.autogen_svn_password'

    self._SVNUploadJsonFiles(src_dir=self.skp_out_dir,
                             dest_subdir='rendered-skps')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareAndUploadWebpageGMs))

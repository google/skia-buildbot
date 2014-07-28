#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# pylint: disable=C0301
"""Uploads Doxygen documentation to Google Storage, where it can be browsed at
http://chromium-skia-gm.commondatastorage.googleapis.com/doxygen/doxygen/html/index.html
"""
# pylint: enable=C0301

import posixpath
import sys

from build_step import BuildStep
from utils import gs_utils
from utils import old_gs_utils
import generate_doxygen
import skia_vars

# It's silly that we include 'doxygen' in the destination path twice, but
# that's to maintain current behavior while fixing http://skbug.com/2658 .
DOXYGEN_GSUTIL_PATH = posixpath.join(
    gs_utils.GSUtils.with_gs_prefix(
        skia_vars.GetGlobalVariable('googlestorage_bucket')),
    'doxygen', 'doxygen')

# Directives for HTTP caching of these files served out by Google Storage.
# See http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html
GS_CACHE_CONTROL_HEADER = 'Cache-Control:public,max-age=3600'


class UploadDoxygen(BuildStep):
  def _Run(self):
    old_gs_utils.upload_dir_contents(
        local_src_dir=generate_doxygen.DOXYGEN_WORKING_DIR,
        remote_dest_dir=DOXYGEN_GSUTIL_PATH,
        gs_acl='public-read',
        http_header_lines=[GS_CACHE_CONTROL_HEADER])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadDoxygen))

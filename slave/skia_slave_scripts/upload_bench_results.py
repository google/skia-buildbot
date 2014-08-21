#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark performance data results. """

from build_step import BuildStep
from utils import sync_bucket_subdir
from utils import old_gs_utils as gs_utils

import gzip
import os
import os.path
import re
import sys
import tempfile
from datetime import datetime


class UploadBenchResults(BuildStep):

  def __init__(self, attempts=5, **kwargs):
    super(UploadBenchResults, self).__init__(attempts=attempts, **kwargs)

  def _UploadJSONResults(self, dest_gsbase, gs_subdir, full_json_path,
                         gzipped=False):
    now = datetime.utcnow()
    gs_json_path = '/'.join((str(now.year).zfill(4), str(now.month).zfill(2),
                            str(now.day).zfill(2), str(now.hour).zfill(2)))
    gs_dir = '/'.join((gs_subdir, gs_json_path, self._builder_name))
    if self._is_try:
      if (not self._args.get('issue_number') or
          self._args['issue_number'] == 'None'):
        raise Exception('issue_number build property is missing!')
      gs_dir = '/'.join(('trybot', gs_dir, self._build_number,
                         self._args['issue_number']))
    full_path_to_upload = full_json_path
    file_to_upload = os.path.basename(full_path_to_upload)
    http_header = ['Content-Type:application/json']
    if gzipped:
      http_header.append('Content-Encoding:gzip')
      gzipped_file = os.path.join(tempfile.gettempdir(), file_to_upload)
      # Apply gzip.
      with open(full_path_to_upload, 'rb') as f_in:
        with gzip.open(gzipped_file, 'wb') as f_out:
          f_out.writelines(f_in)
      full_path_to_upload = gzipped_file
    #TODO(bensong): switch to new gs_utils once it supports http headers.
    gs_utils.upload_file(
        full_path_to_upload,
        '/'.join((dest_gsbase, gs_dir, file_to_upload)),
        gs_acl='public-read',
        http_header_lines=http_header)

  def _RunNanoBenchJSONUpload(self, dest_gsbase):
    """Uploads gzipped nanobench JSON data."""
    # Find the nanobench JSON
    file_list = os.listdir(self._perf_data_dir)
    RE_FILE_SEARCH = re.compile(
        'nanobench_({})_[0-9]+\.json'.format(self._got_revision))
    nanobench_name = None

    for file_name in file_list:
      if RE_FILE_SEARCH.search(file_name):
        nanobench_name = file_name
        break

    if nanobench_name:
      nanobench_json_file = os.path.join(self._perf_data_dir,
                                         nanobench_name)

      self._UploadJSONResults(dest_gsbase, 'nano-json-v1', nanobench_json_file,
                              gzipped=True)

  def _Run(self):
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)
    self._RunNanoBenchJSONUpload(dest_gsbase)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResults))

#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark performance data results. """

from build_step import BuildStep
from utils import sync_bucket_subdir

import json
import os
import os.path
import posixpath
import re
import sys
from datetime import datetime


class UploadBenchResults(BuildStep):

  def __init__(self, attempts=5, **kwargs):
    super(UploadBenchResults, self).__init__(attempts=attempts, **kwargs)

  def _GetPerfDataDir(self):
    return self._perf_data_dir

  def _GetBucketSubdir(self):
    subdirs = ['perfdata', self._builder_name]
    if self._is_try:
      # Trybots need to isolate their results by build number.
      subdirs.append(self._build_number)
    return posixpath.join(*subdirs)

  def _RunInternal(self):
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    # Upload the normal bench logs
    sync_bucket_subdir.SyncBucketSubdir(
        directory=self._GetPerfDataDir(),
        dest_gsbase=dest_gsbase,
        subdir=self._GetBucketSubdir(),
        do_upload=True,
        do_download=False)

    # Find and open JSON file to add in additional fields, then upload.
    json_file_name = None
    file_list = os.listdir(self._GetPerfDataDir())
    for file_name in file_list:
      if re.search('microbench_({})_[0-9]+\.json'.format(self._got_revision),
             file_name):
        json_file_name = os.path.join(self._GetPerfDataDir(), file_name)
        break

    if json_file_name:
      json_data = {}

      with open(json_file_name) as json_file:
        json_data = json.load(json_file)

      json_data['machine'] = self._builder_name
      json_data['commitHash'] = self._got_revision

      with open(json_file_name, 'w') as json_file:
        json.dump(json_data, json_file)

    now = datetime.utcnow()
    gs_json_path = '/'.join((str(now.year).zfill(4), str(now.month).zfill(2),
        str(now.day).zfill(2), str(now.hour).zfill(2)))
    gs_dir = 'stats-json/{}/{}'.format(gs_json_path, self._builder_name)
    sync_bucket_subdir.SyncBucketSubdir(
        directory=self._GetPerfDataDir(),
        dest_gsbase=dest_gsbase,
        subdir=gs_dir,
        # TODO(kelvinly): Set up some way to configure this,
        # rather than hard coding it
        do_upload=True,
        do_download=False,
        filenames_filter=
            'microbench_({})_[0-9]+\.json'.format(self._got_revision))

  def _Run(self):
    self._RunInternal()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResults))

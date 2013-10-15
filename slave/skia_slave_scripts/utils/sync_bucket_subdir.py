#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Synchronize multiple files with a Google Storage Bucket and subdir.
For a lack of a better word 'synchronization' was used, warnings:
1) Downloads files that are not found locally
2) Uploads files that are not found in the bucket
3) Does NOT check version, date or content of the file. If the file is reported
   to be present both locally and in the bucket, no upload nor download is
   performed.
4) If we find filenames that do not match our expectations, an exception
   will be raised
"""

from slave import slave_utils
from utils import download_from_bucket
from utils import upload_to_bucket

import os
import posixpath
import re


DEFAULT_PERFDATA_GS_BASE = 'gs://chromium-skia-gm'
KNOWN_FILENAMES = r'^bench_([0-9a-fr]*)_data.*'
IGNORE_UPLOAD_FILENAMES = ('.DS_Store')


def SyncBucketSubdir(directory, dest_gsbase=DEFAULT_PERFDATA_GS_BASE, subdir='',
    do_upload=True, do_download=True, filenames_filter=KNOWN_FILENAMES,
    min_download_revision=0):
  """ synchronizes a local directory with a cloud one

  dir: directory to synchronize
  dest_gsbase: gs:// bucket to synchronize
  subdir: optional subdirectory within the bucket, multiple directory levels
          are supported, using Unix relative path syntax ("outer/innner")
  do_upload: True to perform upload, False otherwise
  do_download: True to perform download, False otherwise
  filenames_filter: is a regular expression used to match known file names,
                    and re.search(filenames_filter, file_name).group(1)
                    must return revision number.
  min_download_revision: don't transfer files whose revision number
                         (based on filenames_filter) is lower than this
  """

  local_files = set(os.listdir(directory))

  status, output_gsutil_ls = slave_utils.GSUtilListBucket(
      posixpath.join(dest_gsbase, subdir), [])

  # If there is not at least one file in that subdir, gsutil reports error.
  # Writing something like GsUtilExistsSubdir is a lot of pain.
  # We assume that the subdir does not exists, and if there is a real
  # issue, it will surface later.
  if status != 0:
    print 'ls faied'
    output_gsutil_ls = ''

  output_gsutil_ls = set(output_gsutil_ls.splitlines())
  gsbase_subdir = posixpath.join(dest_gsbase, subdir, '')

  cloud_files = set()
  for line in output_gsutil_ls:
    # Ignore lines with warnings and status messages.
    if line.startswith(gsbase_subdir) and line != gsbase_subdir:
      cloud_files.add(line.replace(gsbase_subdir, ''))

  # Download only files not present on the local dir
  if do_download:
    to_download = cloud_files.difference(local_files)
    for file_name in to_download:
      match = re.search(filenames_filter, file_name)
      if not match:
        raise Exception('ERROR: found filename %s on remote filesystem'
                        'that does not match filter %s' % (file_name,
                                                           filenames_filter))
      #TODO(borenet): find alternative ways to handle revision ordering in git.
      #if int(match.group(1)) >= min_download_revision:
      download_from_bucket.DownloadFromBucket(
          posixpath.join(gsbase_subdir, file_name), directory)

  # Uploads only files not present on the cloud storage
  if do_upload:
    to_upload = local_files.difference(cloud_files)
    for file_name in to_upload:
      if file_name not in IGNORE_UPLOAD_FILENAMES:
        match = re.search(filenames_filter, file_name)
        if not match:
          raise Exception('ERROR: %s trying to upload unknown file name.' % (
                          file_name))
        # Ignore force builds without a revision number.
        if match.group(1) != '':
          upload_to_bucket.upload_to_bucket(os.path.join(directory, file_name),
                                            dest_gsbase,
                                            subdir)
  return 0

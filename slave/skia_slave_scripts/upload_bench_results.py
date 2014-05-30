#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark performance data results. """

from build_step import BuildStep
from utils import sync_bucket_subdir

import builder_name_schema

import json
import os
import os.path
import posixpath
import re
import sys
from datetime import datetime


# Modified from Skia repo code, roughly line 108 down of
# bench/check_regressions.py
def ReadExpectations(filename):
  """Reads expectations data from file.

  It returns a dictionary containing tuples of the lower, upper, and expected
  bounds, using the testname and configuration concatenated together as the
  key."""
  expectations = {}
  unstripped_keys = []
  for expectation in open(filename).readlines():
    elements = expectation.strip().split(',')
    if not elements[0] or elements[0].startswith('#'):
      continue
    if len(elements) != 5:
      raise Exception("Invalid expectation line format: %s" %
                      expectation)
    # [<Bench_BmpConfig_TimeType>,<Platform-Alg>]
    bench_entry = elements[0] + ',' + elements[1]
    if bench_entry in unstripped_keys:
      raise Exception("Dup entries for bench expectation %s" %
                      bench_entry)
    unstripped_keys.append(bench_entry)   # Using this in case multiple lines
                                          # share everything except for the
                                          # algorithm and/or platform
    entry = elements[0]
    # [<Bench_BmpConfig_TimeType>] -> (LB, UB, EXPECTED)
    expectations[entry] = (float(elements[-2]),
                           float(elements[-1]),
                           float(elements[-3]))
  return expectations


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
        exclude_json=True,
        do_download=False)

    file_list = os.listdir(self._GetPerfDataDir())
    expectations = {}

    path_to_bench_expectations = os.path.join(
        'expectations',
        'bench',
        'bench_expectations_%s.txt' % builder_name_schema.GetWaterfallBot(
            self._builder_name))
    try:
      expectations = ReadExpectations(path_to_bench_expectations)
    except IOError:
      print "Unable to open expectations file"

    re_file_extract = re.compile(r'(^.*)\.skp.*$')
    json_total = {}
    pictures_timestamp = ''
    for file_name in file_list:
      # Find bench picture files, splice in expectation data
      if not re.search('^bench_{}_data_skp_(.*)_([0-9]*)\.json$'.format(
                   self._got_revision), file_name):
        continue
      json_pictures_data = {}
      if not pictures_timestamp:
        pictures_timestamp = file_name.split('_')[-1].split('.json')[0]
      full_file_name = os.path.join(self._GetPerfDataDir(), file_name)
      with open(full_file_name) as json_pictures:
        print 'Loading file {}'.format(file_name)
        json_pictures_data = json.load(json_pictures)

      if json_total:
        json_total['benches'].extend(json_pictures_data['benches'])
      else:
        json_total = json_pictures_data

    # Now add expectations to all keys
    for bench in json_total['benches']:
      for tileSet in bench['tileSets']:
        search_for_name = re_file_extract.search(bench['name'])
        if not search_for_name:
          print 'Invalid bench name: {}'.format(bench['name'])
          continue
        key = '_'.join([
            search_for_name.group(1)+'.skp',
            tileSet['name'],
            ''])
        if key in expectations.keys():
          (lower, upper, expected) = expectations[key]
          tileSet['lower'] = lower
          tileSet['upper'] = upper
          tileSet['expected'] = expected
        else:
          print "Unable to find key: {}".format(key)

    json_total['commitHash'] = self._got_revision
    json_total['machine'] = self._builder_name

    json_write_name = 'skpbench_{}_{}.json'.format(self._got_revision,
                                                   pictures_timestamp)
    full_json_write_name = os.path.join(self._GetPerfDataDir(), json_write_name)
    with open(full_json_write_name, 'w') as json_picture_write:
      json.dump(json_total, json_picture_write)

    now = datetime.utcnow()
    gs_json_path = '/'.join((str(now.year).zfill(4), str(now.month).zfill(2),
        str(now.day).zfill(2), str(now.hour).zfill(2)))
    gs_dir = 'pics-json/{}/{}'.format(gs_json_path, self._builder_name)
    sync_bucket_subdir.SyncBucketSubdir(
        directory=self._GetPerfDataDir(),
        dest_gsbase=dest_gsbase,
        subdir=gs_dir,
        # TODO(kelvinly): Set up some way to configure this,
        # rather than hard coding it
        do_upload=True,
        do_download=False,
        exclude_json=False,
        filenames_filter=
            'skpbench_({})_[0-9]+\.json'.format(self._got_revision))

    # Find and open the bench JSON file to add in additional fields, then upload.
    microbench_json_file = None

    for file_name in file_list:
      if re.search('microbench_({})_[0-9]+\.json'.format(self._got_revision),
             file_name):
        microbench_json_file = os.path.join(self._GetPerfDataDir(), file_name)
        break

    if microbench_json_file:
      json_data = {}

      with open(microbench_json_file) as json_file:
        json_data = json.load(json_file)

      json_data['machine'] = self._builder_name
      json_data['commitHash'] = self._got_revision

      with open(microbench_json_file, 'w') as json_file:
        json.dump(json_data, json_file)

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
        exclude_json=False,
        filenames_filter=
            'microbench_({})_[0-9]+\.json'.format(self._got_revision))

  def _Run(self):
    self._RunInternal()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResults))

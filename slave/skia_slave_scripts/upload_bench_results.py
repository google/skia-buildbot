#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark performance data results. """

from build_step import BuildStep
from utils import sync_bucket_subdir
from utils import gclient_utils
from utils import upload_to_bucket

import builder_name_schema

import copy
import json
import os
import os.path
import posixpath
import re
import sys
from datetime import datetime


# TODO(kelvinly,borenet): eventually share this code across repos
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
      raise Exception('Invalid expectation line format: {}'.format(
                      expectation))
    # [<Bench_BmpConfig_TimeType>,<Platform-Alg>]
    bench_entry = ','.join(elements[0:1])
    if bench_entry in unstripped_keys:
      raise Exception('Dup entries for bench expectation {}'.format(
                      bench_entry))
    unstripped_keys.append(bench_entry)   # Using this in case multiple lines
                                          # share everything except for the
                                          # algorithm and/or platform
    entry = elements[0]
    # [<Bench_BmpConfig_TimeType>] -> (LB, UB, EXPECTED)
    expectations[entry] = (float(elements[-2]),
                           float(elements[-1]),
                           float(elements[-3]))
  return expectations


def _ParseConfig(config, tileSet=None, idx=0):
  """Converts a config string into a dictionary of configuration values."""
  config_info = config.split('_')
  params = {}
  token_name = None
  for token in config_info:
    if token_name:
      if token_name == 'grid':
        params['bbh'] = 'grid_{}'.format(token)
      else:
        params[token_name] = token
      token_name = None
    elif token in ['scalar', 'viewport', 'grid']:
      token_name = token
      if token == 'scalar':
        token_name = 'scale'
    elif token in ['8888', 'gpu', 'msaa4', 'nvprmsaa4',
                   'nvprmsaa16']:
      params['config'] = token
    elif token in ['rtree', 'quadtree', 'grid']:
      params['bbh'] = token
    elif token in ['simple', 'record', 'tile']:
      params['mode'] = token
      if tileSet and token == 'tile':
        params['mode'] = '_'.join(
            'tile',
            idx % tileSet['tx'],
            idx // tileSet['ty'])
    else:
      if 'badParams' not in params:
        params['badParams'] = token
      else:
        params['badParams'] += '_' + token
  return params



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

  def _RunNormalUpload(self, dest_gsbase):
    # Upload the normal bench logs
    print "Uploading normal data files"
    sync_bucket_subdir.SyncBucketSubdir(
        directory=self._GetPerfDataDir(),
        dest_gsbase=dest_gsbase,
        subdir=self._GetBucketSubdir(),
        do_upload=True,
        exclude_json=True,
        do_download=False)

  def _GetExpectationsPath(self):
    """Gets path to expectations."""
    return os.path.join(
        'expectations',
        'bench',
        'bench_expectations_%s.txt' % builder_name_schema.GetWaterfallBot(
            self._builder_name))


  def _GetTrybotDict(self):
    result = {}
    result['isTrybot'] = builder_name_schema.IsTrybot(self._builder_name)
    return result


  def _UploadResults(self, dest_gsbase, gs_subdir, full_json_path):
    now = datetime.utcnow()
    gs_json_path = '/'.join((str(now.year).zfill(4), str(now.month).zfill(2),
                            str(now.day).zfill(2), str(now.hour).zfill(2)))
    gs_dir = '/'.join((gs_subdir, gs_json_path, self._builder_name))
    upload_to_bucket.upload_to_bucket(
        full_json_path,
        '/'.join((dest_gsbase, gs_dir)))


  def _RunSKPBenchUpload(self, dest_gsbase):
    """Upload SPK bench JSON data, after converting the JSON format."""
    print "Uploading SPK bench JSON data"

    RE_FILE_SEARCH = re.compile(
        '^bench_{}_data_skp_(.*)_([0-9]*)\.json$'.format(self._got_revision))
    file_list = os.listdir(self._GetPerfDataDir())
    expectations = {}
    pictures_timestamp = ''

    try:
      expectations = ReadExpectations(self._GetExpectationsPath())
    except IOError:
      print "Unable to open expectations file"

    json_file_list = []
    for file_name in file_list:
      if RE_FILE_SEARCH.search(file_name):
        json_file_list.append(os.path.join(
            self._GetPerfDataDir(), file_name))

        if not pictures_timestamp:
          pictures_timestamp = file_name.split('_')[-1].split('.json')[0]

    for full_file_name in json_file_list:
      print 'Uploading file %s' % full_file_name
      self._UploadResults(dest_gsbase, 'pics-json-v2', full_file_name)

    if not json_file_list:
      # No data, so no need to upload SKP expectations
      return

    new_json_data = {}
    new_json_data['params'] = builder_name_schema.DictForBuilderName(
        self._builder_name)
    new_json_data['params']['builderName'] = self._builder_name
    new_json_data['buildNumber'] = int(self._build_number)
    new_json_data['timestamp'] = gclient_utils.GetGitRepoPOSIXTimestamp()
    new_json_data['gitHash'] = self._got_revision
    new_json_data['gitNumber'] = gclient_utils.GetGitNumber(self._got_revision)
    # NOTE: site_config/build_name_schema.py also affects the schema

    # Get trybot params
    new_json_data.update(self._GetTrybotDict())

    # Load in bench data
    json_write_name = 'skpbench_%s_%s.json' % (self._got_revision,
                                               pictures_timestamp)
    full_json_write_name = os.path.join(self._GetPerfDataDir(), json_write_name)
    with open(full_json_write_name, 'w') as json_picture_write:
      # Load in expectations data
      TYPE_DICT = {'': 'wall', 'c': 'cpu', 'g': 'gpu'}
      for key in expectations.keys():
        for idx, name in [(0, 'lower'), (1, 'upper'), (2, 'expected')]:
          new_row = copy.deepcopy(new_json_data)
          stripped_bench_name = '{}.skp'.format(key.split('.skp')[0])
          config_info = key.split('.skp')[1].split('_')
          new_row['key'] = self._builder_name + '_' + key + name
          new_row['value'] = expectations[key][idx]
          new_row['params']['benchName'] = stripped_bench_name
          new_row['params']['measurementType'] = '_'.join([
              name,
              TYPE_DICT[config_info[-1]]])
          new_params = _ParseConfig(key.split('.skp')[1])
          new_row['params'].update(new_params)
          json.dump(new_row, json_picture_write)
          json_picture_write.write('\n')

    self._UploadResults(dest_gsbase, 'pics-json-v2', full_json_write_name)


  def _RunBenchUpload(self, dest_gsbase):
    """Uploads bench JSON data, after modifying the format."""
    # Find and open the bench JSON file to add in additional fields, then upload
    file_list = os.listdir(self._GetPerfDataDir())
    RE_FILE_SEARCH = re.compile(
        'microbench_({})_[0-9]+\.json'.format(self._got_revision))
    microbench_name = None

    for file_name in file_list:
      if RE_FILE_SEARCH.search(file_name):
        microbench_name = file_name
        break

    if microbench_name:
      microbench_json_file = os.path.join(self._GetPerfDataDir(),
                                          microbench_name)
      json_data = {}

      with open(microbench_json_file) as json_file:
        json_data = json.load(json_file)

      if not json_data:
        print 'Unable to read JSON bench data'
        return

      new_json_data = {}
      new_json_data['params'] = builder_name_schema.DictForBuilderName(
          self._builder_name)
      new_json_data['params']['builderName'] = self._builder_name
      new_json_data['params']['scale'] = 1.0
      new_json_data['buildNumber'] = int(self._build_number)
      new_json_data['timestamp'] = gclient_utils.GetGitRepoPOSIXTimestamp()
      new_json_data['gitHash'] = self._got_revision
      new_json_data['gitNumber'] = gclient_utils.GetGitNumber(
          self._got_revision)

      # Get trybot params
      new_json_data.update(self._GetTrybotDict())

      for key in json_data['options'].keys():
        new_json_data['params'][key] = json_data['options'][key]

      options_list = []
      for key in sorted(json_data['options'].keys()):
        options_list.append('%s_%s' % (key, json_data['options'][key]))

      options_key = '_'.join(options_list)
      key_base = '_'.join([
          self._builder_name,
          json_data['options']['system'],
          options_key])

      # Choose a different name here so it doesn't overwrite the old file
      json_write_name = microbench_json_file.replace(
          'microbench',
          'microbench2')
      with open(json_write_name, 'w') as json_write_file:
        for result in json_data['results']:
          for values in result['results']:
            for measurement in ['cmsecs', 'gmsecs', 'msecs']:
              if measurement in values.keys():
                key = '_'.join([
                    key_base,
                    result['name'],
                    values['name'],
                    measurement])
                new_row = new_json_data
                new_row['key'] = key
                new_row['value'] = values[measurement]
                new_row['params']['gpuConfig'] = values['name']
                new_row['params']['testName'] = result['name']
                new_row['params']['measurementType'] = measurement
                json.dump(new_row, json_write_file)
                json_write_file.write('\n')

      self._UploadResults(dest_gsbase, 'stats-json-v2', json_write_name)

  def _RunInternal(self):
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)
    self._RunNormalUpload(dest_gsbase)
    self._RunBenchUpload(dest_gsbase)
    self._RunSKPBenchUpload(dest_gsbase)


  def _Run(self):
    self._RunInternal()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResults))

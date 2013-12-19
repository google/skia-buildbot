#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that outputs an JSON summary containing the comparision of images."""

import imp
import json
import optparse
import os
import posixpath
import sys

sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir))
import json_summary_constants


def WriteJsonSummary(nopatch_json, withpatch_json, output_file_path,
                     gs_output_dir, gs_skp_dir, slave_num, gm_json_path):
  """Outputs the JSON summary of image comparisions.

  Args:
    nopatch_json: (str) Location of the nopatch render_pictures JSON summary
        file.
    withpatch_json: (str) Location of the withpatch render_pictures JSON summary
        file.
    output_file_path: (str) The local path to the JSON file that will be
        created by this function which will contain a summary of all file
        differences for this slave.
    gs_output_dir: (str) The directory the JSON summary file and images will be
        outputted to in Google Storage.
    gs_skp_dir: (str) The Google Storage directory that contains the SKPs of
        this cluster telemetry slave.
    slave_num: (str) The number of the cluster telemetry slave that is running
        this script.
    gm_json_path: (str) Local complete path to gm_json.py in Skia trunk.
  """

  assert os.path.isfile(gm_json_path), 'Must specify a valid path to gm_json.py'
  gm_json_file = os.path.basename(gm_json_path)
  gm_json_mod = imp.load_source(gm_json_file, gm_json_path)

  files_to_checksums1 =  GetFilesAndChecksums(nopatch_json, gm_json_mod)
  files_to_checksums2 =  GetFilesAndChecksums(withpatch_json, gm_json_mod)

  assert len(files_to_checksums1) == len(files_to_checksums2), (
      'Number of images in both JSON summary files are different')
  assert files_to_checksums1.keys() == files_to_checksums2.keys(), (
      'File names in both JSON summary files are different')

  # Compare checksums in both directories and output differences.
  file_differences = []
  slave_dict = {
      json_summary_constants.JSONKEY_SKPS_LOCATION: gs_skp_dir,
      json_summary_constants.JSONKEY_FAILED_FILES: file_differences,
      json_summary_constants.JSONKEY_FILES_LOCATION_NOPATCH: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'nopatch-images'),
      json_summary_constants.JSONKEY_FILES_LOCATION_WITHPATCH: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'withpatch-images')
  }
  json_summary = {
      'slave%s' % slave_num: slave_dict
  }
  for file1 in files_to_checksums1:
    algo1, checksum1 = files_to_checksums1[file1]
    algo2, checksum2 = files_to_checksums2[file1]
    assert algo1 == algo2, 'Different algorithms found'
    if checksum1 != checksum2:
      file_differences.append({
          json_summary_constants.JSONKEY_FILE_NAME: file1,
          json_summary_constants.JSONKEY_SKP_LOCATION: posixpath.join(
              gs_skp_dir, GetSkpFileName(file1))
      })
  if file_differences:
    slave_dict[json_summary_constants.JSONKEY_FAILED_FILES_COUNT] = len(
        file_differences)
    with open(output_file_path, 'w') as f:
      f.write(json.dumps(json_summary, indent=4, sort_keys=True))


def GetSkpFileName(img_file_name):
  """Determine the SKP file name from the image's file name."""
  # TODO(rmistry): The below relies too much on the current output of render
  # pictures to determine the root SKP.
  return '%s_.skp' % '_'.join(img_file_name.split('_')[:-1])


def GetFilesAndChecksums(json_location, gm_json_mod):
  """Reads the JSON summary and returns dict of files to checksums."""
  data = gm_json_mod.LoadFromFile(json_location)
  if data:
    return data[gm_json_mod.JSONKEY_ACTUALRESULTS][
        gm_json_mod.JSONKEY_ACTUALRESULTS_NOCOMPARISON]
  else:
    return {}


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--nopatch_json',
      help='Location of the nopatch render_pictures JSON summary file.')
  option_parser.add_option(
      '', '--withpatch_json',
      help='Location of the withpatch render_pictures JSON summary file.')
  option_parser.add_option(
      '', '--output_file_path',
      help='The local path to the JSON file that will be created by this '
           'script which will contain a summary of all file differences for '
           'this slave.')
  option_parser.add_option(
      '', '--gs_output_dir',
      help='The directory the JSON summary file and images will be outputted '
           'to in Google Storage.')
  option_parser.add_option(
      '', '--gs_skp_dir',
      help='The Google Storage directory that contains the SKPs of this '
           'cluster telemetry slave.')
  option_parser.add_option(
      '', '--slave_num',
      help='The number of the cluster telemetry slave that is running this '
           'script.')
  option_parser.add_option(
      '', '--gm_json_path',
      help='Local complete path to gm_json.py in Skia trunk.')
  options, unused_args = option_parser.parse_args()
  if (not options.nopatch_json or not options.withpatch_json
      or not options.output_file_path or not options.gs_output_dir
      or not options.gs_skp_dir or not options.slave_num
      or not options.gm_json_path):
    option_parser.error(
        'Must specify nopatch_json, withpatch_json2, output_file_path, '
        'gs_output_dir, gs_skp_dir, slave_num and gm_json_path.')

  WriteJsonSummary(options.nopatch_json, options.withpatch_json, 
                   options.output_file_path, options.gs_output_dir,
                   options.gs_skp_dir, options.slave_num, options.gm_json_path)

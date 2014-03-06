#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that outputs an JSON summary containing the comparision of images."""

import json
import optparse
import os
import posixpath
import sys
import traceback

sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir))
import json_summary_constants

# Modules from skia's gm/ and gm/rebaseline_server/ dirs.
try:
  import gm_json
  import imagediffdb
except ImportError:
  print 'sys.path is [%s]' % sys.path
  traceback.print_exc()
  raise Exception('You need to add gm/ and gm/rebaseline_server to PYTHONPATH')


def WriteJsonSummary(img_root, nopatch_json, nopatch_images_base_url,
                     withpatch_json, withpatch_images_base_url,
                     output_file_path, gs_output_dir, gs_skp_dir, slave_num):
  """Outputs the JSON summary of image comparisions.

  Args:
    img_root: (str) The root directory on local disk where we store all images.
    nopatch_json: (str) Location of the nopatch render_pictures JSON summary
        file.
    nopatch_images_base_url: (str) URL of directory containing all nopatch
        images.
    withpatch_json: (str) Location of the withpatch render_pictures JSON summary
        file.
    withpatch_images_base_url: (str) URL of directory containing all withpatch
        images.
    output_file_path: (str) The local path to the JSON file that will be
        created by this function which will contain a summary of all file
        differences for this slave.
    gs_output_dir: (str) The directory the JSON summary file and images will be
        outputted to in Google Storage.
    gs_skp_dir: (str) The Google Storage directory that contains the SKPs of
        this cluster telemetry slave.
    slave_num: (str) The number of the cluster telemetry slave that is running
        this script.
  """
  files_to_checksums_nopatch = GetFilesAndChecksums(nopatch_json)
  files_to_checksums_withpatch = GetFilesAndChecksums(withpatch_json)

  assert len(files_to_checksums_nopatch) == len(files_to_checksums_withpatch), (
      'Number of images in both JSON summary files are different')
  assert files_to_checksums_nopatch.keys() == \
         files_to_checksums_withpatch.keys(), (
             'File names in both JSON summary files are different')

  # Compare checksums in both directories and output differences.
  file_differences = []
  slave_dict = {
      json_summary_constants.JSONKEY_SKPS_LOCATION: gs_skp_dir,
      json_summary_constants.JSONKEY_FAILED_FILES: file_differences,
      json_summary_constants.JSONKEY_FILES_LOCATION_NOPATCH: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'nopatch-images'),
      json_summary_constants.JSONKEY_FILES_LOCATION_WITHPATCH: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'withpatch-images'),
      json_summary_constants.JSONKEY_FILES_LOCATION_DIFFS: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'diffs'),
      json_summary_constants.JSONKEY_FILES_LOCATION_WHITE_DIFFS: posixpath.join(
          gs_output_dir, 'slave%s' % slave_num, 'whitediffs')
  }
  json_summary = {
      'slave%s' % slave_num: slave_dict
  }

  image_diff_db = imagediffdb.ImageDiffDB(storage_root=img_root)
  for filename in files_to_checksums_nopatch:
    algo_nopatch, checksum_nopatch = files_to_checksums_nopatch[filename]
    algo_withpatch, checksum_withpatch = files_to_checksums_withpatch[filename]
    assert algo_nopatch == algo_withpatch, 'Different checksum algorithms found'
    if checksum_nopatch != checksum_withpatch:
      # TODO(epoger): It seems silly that we add this DiffRecord to ImageDiffDB
      # and then pull it out again right away, but this is a stepping-stone
      # to using ImagePairSet instead of replicating its behavior here.
      image_locator_base = os.path.splitext(filename)[0]
      image_locator_nopatch = image_locator_base + '_nopatch'
      image_locator_withpatch = image_locator_base + '_withpatch'
      image_diff_db.add_image_pair(
          expected_image_url=posixpath.join(nopatch_images_base_url, filename),
          expected_image_locator=image_locator_nopatch,
          actual_image_url=posixpath.join(withpatch_images_base_url, filename),
          actual_image_locator=image_locator_withpatch)
      diff_record = image_diff_db.get_diff_record(
          expected_image_locator=image_locator_nopatch,
          actual_image_locator=image_locator_withpatch)
      file_differences.append({
          json_summary_constants.JSONKEY_FILE_NAME: filename,
          json_summary_constants.JSONKEY_SKP_LOCATION: posixpath.join(
              gs_skp_dir, GetSkpFileName(filename)),
          json_summary_constants.JSONKEY_NUM_PIXELS_DIFFERING:
              diff_record.get_num_pixels_differing(),
          json_summary_constants.JSONKEY_PERCENT_PIXELS_DIFFERING:
              diff_record.get_percent_pixels_differing(),
          json_summary_constants.JSONKEY_WEIGHTED_DIFF_MEASURE:
              diff_record.get_weighted_diff_measure(),
          json_summary_constants.JSONKEY_MAX_DIFF_PER_CHANNEL:
              diff_record.get_max_diff_per_channel(),
          json_summary_constants.JSONKEY_PERCEPTUAL_DIFF:
              diff_record.get_perceptual_difference(),
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


def GetFilesAndChecksums(json_location):
  """Reads the JSON summary and returns dict of files to checksums."""
  data = gm_json.LoadFromFile(json_location)
  if data:
    return data[gm_json.JSONKEY_ACTUALRESULTS][
        gm_json.JSONKEY_ACTUALRESULTS_NOCOMPARISON]
  else:
    return {}


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--img_root',
      help='The root directory on local disk where we store all images.')
  option_parser.add_option(
      '', '--nopatch_json',
      help='Location of the nopatch render_pictures JSON summary file.')
  option_parser.add_option(
      '', '--nopatch_images_base_url',
      help='URL of directory containing all nopatch images.')
  option_parser.add_option(
      '', '--withpatch_json',
      help='Location of the withpatch render_pictures JSON summary file.')
  option_parser.add_option(
      '', '--withpatch_images_base_url',
      help='URL of directory containing all withpatch images.')
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
  options, unused_args = option_parser.parse_args()
  if (not options.nopatch_json or not options.withpatch_json
      or not options.output_file_path or not options.gs_output_dir
      or not options.gs_skp_dir or not options.slave_num
      or not options.img_root
      or not options.nopatch_images_base_url
      or not options.withpatch_images_base_url):
    option_parser.error(
        'Must specify img_root, nopatch_json, nopatch_images_base_url, '
        'withpatch_json, withpatch_images_base_url, output_file_path, '
        'gs_output_dir, gs_skp_dir, and slave_num.')

  WriteJsonSummary(options.img_root, options.nopatch_json,
                   options.nopatch_images_base_url, options.withpatch_json,
                   options.withpatch_images_base_url, options.output_file_path,
                   options.gs_output_dir, options.gs_skp_dir, options.slave_num)

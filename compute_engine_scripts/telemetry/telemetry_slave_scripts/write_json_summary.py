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

# TODO(epoger): These constants must be kept in sync with the ones in
# https://skia.googlesource.com/skia/+/master/tools/PictureRenderer.cpp
JSONKEY_HEADER = 'header'
JSONKEY_HEADER_TYPE = 'type'
JSONKEY_HEADER_REVISION = 'revision'
JSONKEY_IMAGE_CHECKSUMALGORITHM = 'checksumAlgorithm'
JSONKEY_IMAGE_CHECKSUMVALUE = 'checksumValue'
JSONKEY_IMAGE_COMPARISONRESULT = 'comparisonResult'
JSONKEY_IMAGE_FILEPATH = 'filepath'
JSONKEY_SOURCE_TILEDIMAGES = 'tiled-images'
JSONKEY_SOURCE_WHOLEIMAGE = 'whole-image'

JSONVALUE_HEADER_TYPE = 'ChecksummedImages'
JSONVALUE_HEADER_REVISION = 1

IMAGE_SOURCE = 'imageSource'


def WriteJsonSummary(img_root, nopatch_json, nopatch_images_base_url,
                     withpatch_json, withpatch_images_base_url,
                     output_file_path, gs_output_dir, gs_skp_dir, slave_num,
                     additions_to_sys_path):
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
    additions_to_sys_path: ([str]) A list of path components to add to sys.path;
        typically used to provide rebaseline_server Python modules.
  """
  for dirpath in additions_to_sys_path:
    if dirpath not in sys.path:
      sys.path.append(dirpath)

  # Modules from skia's gm/ and gm/rebaseline_server/ dirs.
  try:
    import gm_json
    import imagediffdb
  except ImportError:
    print 'sys.path is [%s]' % sys.path
    traceback.print_exc()
    raise Exception('You need to add gm/ and gm/rebaseline_server to sys.path')

  all_image_descriptions_nopatch = GetImageDescriptions(gm_json, nopatch_json)
  all_image_descriptions_withpatch = GetImageDescriptions(
      gm_json, withpatch_json)

  assert (len(all_image_descriptions_nopatch) ==
          len(all_image_descriptions_withpatch)), \
          'Number of images in the two JSON summary files are different'
  assert (all_image_descriptions_nopatch.keys() ==
          all_image_descriptions_withpatch.keys()), \
          'SKP filenames in the two JSON summary files are different'

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
  for image_filepath in all_image_descriptions_nopatch:
    image_desc_nopatch = all_image_descriptions_nopatch[image_filepath]
    image_desc_withpatch = all_image_descriptions_withpatch[image_filepath]

    algo_nopatch = image_desc_nopatch[JSONKEY_IMAGE_CHECKSUMALGORITHM]
    algo_withpatch = image_desc_withpatch[JSONKEY_IMAGE_CHECKSUMALGORITHM]
    assert algo_nopatch == algo_withpatch, 'Different checksum algorithms'

    imagefile_nopatch = image_desc_nopatch[JSONKEY_IMAGE_FILEPATH]
    imagefile_withpatch = image_desc_withpatch[JSONKEY_IMAGE_FILEPATH]
    assert imagefile_nopatch == imagefile_withpatch, 'Different imagefile names'

    skpfile_nopatch = image_desc_nopatch[IMAGE_SOURCE]
    skpfile_withpatch = image_desc_withpatch[IMAGE_SOURCE]
    assert skpfile_nopatch == skpfile_withpatch, 'Different skpfile names'

    checksum_nopatch = image_desc_nopatch[JSONKEY_IMAGE_CHECKSUMVALUE]
    checksum_withpatch = image_desc_withpatch[JSONKEY_IMAGE_CHECKSUMVALUE]
    if checksum_nopatch != checksum_withpatch:
      # TODO(epoger): It seems silly that we add this DiffRecord to ImageDiffDB
      # and then pull it out again right away, but this is a stepping-stone
      # to using ImagePairSet instead of replicating its behavior here.
      image_locator_base = os.path.splitext(imagefile_nopatch)[0]
      image_locator_nopatch = image_locator_base + '_nopatch'
      image_locator_withpatch = image_locator_base + '_withpatch'
      image_diff_db.add_image_pair(
          expected_image_url=posixpath.join(
              nopatch_images_base_url, image_filepath),
          expected_image_locator=image_locator_nopatch,
          actual_image_url=posixpath.join(
              withpatch_images_base_url, image_filepath),
          actual_image_locator=image_locator_withpatch)
      diff_record = image_diff_db.get_diff_record(
          expected_image_locator=image_locator_nopatch,
          actual_image_locator=image_locator_withpatch)
      file_differences.append({
          json_summary_constants.JSONKEY_FILE_NAME: imagefile_nopatch,
          json_summary_constants.JSONKEY_SKP_LOCATION: posixpath.join(
              gs_skp_dir, skpfile_nopatch),
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


def GetImageDescriptions(gm_json_mod, json_location):
  """Reads the JSON summary and returns {ImageFilePath: ImageDescription} dict.

  Each ImageDescription is a dict of this form:
  {
    JSONKEY_IMAGE_CHECKSUMALGORITHM: 'bitmap-64bitMD5',
    JSONKEY_IMAGE_CHECKSUMVALUE: 5815827069051002745,
    JSONKEY_IMAGE_COMPARISONRESULT: 'no-comparison',
    JSONKEY_IMAGE_FILEPATH: 'red_skp-tile0.png', # equals ImageFilePath dict key
    IMAGE_SOURCE: 'red.skp'
  }
  """
  json_data = gm_json_mod.LoadFromFile(json_location)
  if json_data:
    header_type = json_data[JSONKEY_HEADER][JSONKEY_HEADER_TYPE]
    if header_type != JSONVALUE_HEADER_TYPE:
      raise Exception('expected header_type %s but found %s' % (
          JSONVALUE_HEADER_TYPE, header_type))
    header_revision = json_data[JSONKEY_HEADER][JSONKEY_HEADER_REVISION]
    if header_revision != JSONVALUE_HEADER_REVISION:
      raise Exception('expected header_revision %s but found %s' % (
          JSONVALUE_HEADER_REVISION, header_revision))

    actual_results = json_data[gm_json_mod.JSONKEY_ACTUALRESULTS]
    newdict = {}
    for skp_file in actual_results:
      whole_image_description = actual_results[skp_file].get(
          JSONKEY_SOURCE_WHOLEIMAGE, None)
      all_image_descriptions = actual_results[skp_file].get(
          JSONKEY_SOURCE_TILEDIMAGES, [])
      if whole_image_description:
        all_image_descriptions.append(whole_image_description)
      for image_description in all_image_descriptions:
        image_filepath = image_description[JSONKEY_IMAGE_FILEPATH]
        image_description[IMAGE_SOURCE] = skp_file
        if image_filepath in newdict:
          raise Exception('found two images with same filepath %s' %
                          image_filepath)
        newdict[image_filepath] = image_description
    return newdict
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
  option_parser.add_option(
      '', '--add_to_sys_path',
      action='append',
      help='Directory to add to sys.path.  May be repeated.')
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
                   options.gs_output_dir, options.gs_skp_dir, options.slave_num,
                   options.add_to_sys_path)

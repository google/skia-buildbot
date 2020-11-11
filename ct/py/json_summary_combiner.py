#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that combines JSON summaries and outputs the summaries in HTML."""

import glob
import json
import optparse
import os
import posixpath
import sys

sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir))
import json_summary_constants

# Add the django settings file to DJANGO_SETTINGS_MODULE.
import django
os.environ['DJANGO_SETTINGS_MODULE'] = 'csv-django-settings'
django.setup()
from django.template import loader

STORAGE_HTTP_BASE = 'http://storage.cloud.google.com'


# Template variables used in the django templates defined in django-settings.
# If the values of these constants change then the django templates need to
# change as well.
WORKER_NAME_TO_INFO_ITEMS_TEMPLATE_VAR = 'worker_name_to_info_items'
ABSOLUTE_URL_TEMPLATE_VAR = 'absolute_url'
WORKER_INFO_TEMPLATE_VAR = 'worker_info'
FILE_INFO_TEMPLATE_VAR = 'file_info'
RENDER_PICTURES_ARGS_TEMPLATE_VAR = 'render_pictures_args'
NOPATCH_GPU_TEMPLATE_VAR = 'nopatch_gpu'
WITHPATCH_GPU_TEMPLATE_VAR = 'withpatch_gpu'
TOTAL_FAILING_FILES_TEMPLATE_VAR = 'failing_files_count'
GS_FILES_LOCATION_NO_PATCH_TEMPLATE_VAR = 'gs_http_files_location_nopatch'
GS_FILES_LOCATION_WITH_PATCH_TEMPLATE_VAR = 'gs_http_files_location_withpatch'
GS_FILES_LOCATION_DIFFS_TEMPLATE_VAR = 'gs_http_files_location_diffs'
GS_FILES_LOCATION_WHITE_DIFFS_TEMPLATE_VAR = 'gs_http_files_location_whitediffs'


class FileInfo(object):
  """Container class that holds all file data."""
  def __init__(self, file_name, skp_location, num_pixels_differing,
               percent_pixels_differing,
               max_diff_per_channel, perceptual_diff):
    self.file_name = file_name
    self.diff_file_name = _GetDiffFileName(self.file_name)
    self.skp_location = skp_location
    self.num_pixels_differing = num_pixels_differing
    self.percent_pixels_differing = percent_pixels_differing
    self.max_diff_per_channel = max_diff_per_channel
    self.perceptual_diff = perceptual_diff


def _GetDiffFileName(file_name):
  file_name_no_ext, ext = os.path.splitext(file_name)
  ext = ext.lstrip('.')
  return '%s_nopatch_%s-vs-%s_withpatch_%s.%s' % (
      file_name_no_ext, ext, file_name_no_ext, ext, ext)


class WorkerInfo(object):
  """Container class that holds all worker data."""
  def __init__(self, worker_name, failed_files, skps_location,
               files_location_nopatch, files_location_withpatch,
               files_location_diffs, files_location_whitediffs):
    self.worker_name = worker_name
    self.failed_files = failed_files
    self.files_location_nopatch = files_location_nopatch
    self.files_location_withpatch = files_location_withpatch
    self.files_location_diffs = files_location_diffs
    self.files_location_whitediffs = files_location_whitediffs
    self.skps_location = skps_location


def CombineJsonSummaries(json_summaries_dir):
  """Combines JSON summaries and returns the summaries in HTML."""
  worker_name_to_info = {}
  for json_summary in glob.glob(os.path.join(json_summaries_dir, '*.json')):
    with open(json_summary) as f:
      data = json.load(f)
    # There must be only one top level key and it must be the worker name.
    assert len(data.keys()) == 1

    worker_name = data.keys()[0]
    worker_data = data[worker_name]
    file_info_list = []
    for failed_file in worker_data[json_summary_constants.JSONKEY_FAILED_FILES]:
      failed_file_name = failed_file[json_summary_constants.JSONKEY_FILE_NAME]
      skp_location = posixpath.join(
          STORAGE_HTTP_BASE,
          failed_file[
              json_summary_constants.JSONKEY_SKP_LOCATION].lstrip('gs://'))
      num_pixels_differing = failed_file[
          json_summary_constants.JSONKEY_NUM_PIXELS_DIFFERING]
      percent_pixels_differing = failed_file[
          json_summary_constants.JSONKEY_PERCENT_PIXELS_DIFFERING]
      max_diff_per_channel = failed_file[
          json_summary_constants.JSONKEY_MAX_DIFF_PER_CHANNEL]
      perceptual_diff = failed_file[
          json_summary_constants.JSONKEY_PERCEPTUAL_DIFF]

      file_info = FileInfo(
          file_name=failed_file_name,
          skp_location=skp_location,
          num_pixels_differing=num_pixels_differing,
          percent_pixels_differing=percent_pixels_differing,
          max_diff_per_channel=max_diff_per_channel,
          perceptual_diff=perceptual_diff)
      file_info_list.append(file_info)

    worker_info = WorkerInfo(
        worker_name=worker_name,
        failed_files=file_info_list,
        skps_location=worker_data[json_summary_constants.JSONKEY_SKPS_LOCATION],
        files_location_nopatch=worker_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_NOPATCH],
        files_location_withpatch=worker_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_WITHPATCH],
        files_location_diffs=worker_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_DIFFS],
        files_location_whitediffs=worker_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_WHITE_DIFFS])
    worker_name_to_info[worker_name] = worker_info

  return worker_name_to_info


def OutputToHTML(worker_name_to_info, output_html_dir, absolute_url,
                 render_pictures_args, nopatch_gpu, withpatch_gpu):
  """Outputs a worker name to WorkerInfo dict into HTML.

  Creates a top level HTML file that lists worker names to the number of failing
  files. Also creates X number of HTML files that lists all the failing files
  and displays the nopatch and withpatch images. X here corresponds to the
  number of workers that have failing files.
  """
  # Get total failing file count.
  total_failing_files = 0
  for worker_info in worker_name_to_info.values():
    total_failing_files += len(worker_info.failed_files)

  worker_name_to_info_items = sorted(
      worker_name_to_info.items(), key=lambda tuple: tuple[0])
  rendered = loader.render_to_string(
      'workers_totals.html',
      {WORKER_NAME_TO_INFO_ITEMS_TEMPLATE_VAR: worker_name_to_info_items,
       ABSOLUTE_URL_TEMPLATE_VAR: absolute_url,
       RENDER_PICTURES_ARGS_TEMPLATE_VAR: render_pictures_args,
       NOPATCH_GPU_TEMPLATE_VAR: nopatch_gpu,
       WITHPATCH_GPU_TEMPLATE_VAR: withpatch_gpu,
       TOTAL_FAILING_FILES_TEMPLATE_VAR: total_failing_files}
  )
  with open(os.path.join(output_html_dir, 'index.html'), 'wb') as index_html:
    index_html.write(rendered)

  rendered = loader.render_to_string(
      'list_of_all_files.html',
      {WORKER_NAME_TO_INFO_ITEMS_TEMPLATE_VAR: worker_name_to_info_items,
       ABSOLUTE_URL_TEMPLATE_VAR: absolute_url}
  )
  with open(os.path.join(output_html_dir,
                         'list_of_all_files.html'), 'wb') as files_html:
    files_html.write(rendered)

  for worker_info in worker_name_to_info.values():
    for file_info in worker_info.failed_files:
      rendered = loader.render_to_string(
          'single_file_details.html',
          {FILE_INFO_TEMPLATE_VAR: file_info,
         ABSOLUTE_URL_TEMPLATE_VAR: absolute_url,
         GS_FILES_LOCATION_NO_PATCH_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             worker_info.files_location_nopatch.lstrip('gs://')),
         GS_FILES_LOCATION_WITH_PATCH_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             worker_info.files_location_withpatch.lstrip('gs://')),
         GS_FILES_LOCATION_DIFFS_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             worker_info.files_location_diffs.lstrip('gs://')),
         GS_FILES_LOCATION_WHITE_DIFFS_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             worker_info.files_location_whitediffs.lstrip('gs://'))}
      )
      with open(os.path.join(output_html_dir, '%s.html' % file_info.file_name),
                'wb') as per_file_html:
        per_file_html.write(rendered)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--json_summaries_dir',
      help='Location of JSON summary files from all GCE workers.')
  option_parser.add_option(
      '', '--output_html_dir',
      help='The absolute path of the HTML dir that will contain the results of'
           ' this script.')
  option_parser.add_option(
      '', '--absolute_url',
      help='Servers like Google Storage require an absolute url for links '
           'within the HTML output files.',
      default='')
  option_parser.add_option(
      '', '--render_pictures_args',
      help='The arguments specified by the user to the render_pictures binary.')
  option_parser.add_option(
      '', '--nopatch_gpu',
      help='Specifies whether the nopatch render_pictures run was done with '
           'GPU.')
  option_parser.add_option(
      '', '--withpatch_gpu',
      help='Specifies whether the withpatch render_pictures run was done with '
           'GPU.')
  options, unused_args = option_parser.parse_args()
  if (not options.json_summaries_dir or not options.output_html_dir
      or not options.render_pictures_args or not options.nopatch_gpu
      or not options.withpatch_gpu):
    option_parser.error(
        'Must specify json_summaries_dir, output_html_dir, '
        'render_pictures_args, nopatch_gpu and withpatch_gpu')

  OutputToHTML(
      worker_name_to_info=CombineJsonSummaries(options.json_summaries_dir),
      output_html_dir=options.output_html_dir,
      absolute_url=options.absolute_url,
      render_pictures_args=options.render_pictures_args,
      nopatch_gpu=options.nopatch_gpu,
      withpatch_gpu=options.withpatch_gpu)

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

from django.template import loader

sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir))
import json_summary_constants

# Add the django settings file to DJANGO_SETTINGS_MODULE.
os.environ['DJANGO_SETTINGS_MODULE'] = 'django-settings'

STORAGE_HTTP_BASE = 'http://storage.cloud.google.com'


# Template variables used in the django templates defined in django-settings.
# If the values of these constants change then the django templates need to
# change as well.
SLAVE_NAME_TO_INFO_ITEMS_TEMPLATE_VAR = 'slave_name_to_info_items'
ABSOLUTE_URL_TEMPLATE_VAR = 'absolute_url'
SLAVE_INFO_TEMPLATE_VAR = 'slave_info'
GS_FILES_LOCATION_NO_PATCH_TEMPLATE_VAR = 'gs_http_files_location_nopatch'
GS_FILES_LOCATION_WITH_PATCH_TEMPLATE_VAR = 'gs_http_files_location_withpatch'


class SlaveInfo(object):
  """Container class that holds all slave data."""
  def __init__(self, slave_name, failed_files, skps_location,
               files_location_nopatch, files_location_withpatch):
    self.slave_name = slave_name
    self.failed_files = failed_files
    self.files_location_nopatch = files_location_nopatch
    self.files_location_withpatch = files_location_withpatch
    self.skps_location = skps_location


def CombineJsonSummaries(json_summaries_dir):
  """Combines JSON summaries and returns the summaries in HTML."""
  slave_name_to_info = {}
  for json_summary in glob.glob(os.path.join(json_summaries_dir, '*.json')):
    with open(json_summary) as f:
      data = json.load(f)
    # There must be only one top level key and it must be the slave name.
    assert len(data.keys()) == 1

    slave_name = data.keys()[0]
    slave_data = data[slave_name]
    failed_files = []
    for failed_file in slave_data[json_summary_constants.JSONKEY_FAILED_FILES]:
      failed_files.extend(failed_file.values())
    slave_info = SlaveInfo(
        slave_name=slave_name,
        failed_files=failed_files,
        skps_location=slave_data[json_summary_constants.JSONKEY_SKPS_LOCATION],
        files_location_nopatch=slave_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_NOPATCH],
        files_location_withpatch=slave_data[
            json_summary_constants.JSONKEY_FILES_LOCATION_WITHPATCH])
    slave_name_to_info[slave_name] = slave_info

  return slave_name_to_info


def OutputToHTML(slave_name_to_info, output_html_dir, absolute_url):
  """Outputs a slave name to SlaveInfo dict into HTML.

  Creates a top level HTML file that lists slave names to the number of failing
  files. Also creates X number of HTML files that lists all the failing files
  and displays the nopatch and withpatch images. X here corresponds to the
  number of slaves that have failing files.
  """
  slave_name_to_info_items = sorted(
      slave_name_to_info.items(), key=lambda tuple: tuple[0])
  rendered = loader.render_to_string(
      'slaves_totals.html',
       {SLAVE_NAME_TO_INFO_ITEMS_TEMPLATE_VAR: slave_name_to_info_items,
        ABSOLUTE_URL_TEMPLATE_VAR: absolute_url}
  )
  with open(os.path.join(output_html_dir, 'index.html'), 'w') as index_html:
    index_html.write(rendered)

  for slave_name, slave_info in slave_name_to_info_items:
    rendered = loader.render_to_string(
        'failures_per_slave.html',
        {SLAVE_INFO_TEMPLATE_VAR: slave_info,
         ABSOLUTE_URL_TEMPLATE_VAR: absolute_url,
         GS_FILES_LOCATION_NO_PATCH_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             slave_info.files_location_nopatch.lstrip('gs://')),
         GS_FILES_LOCATION_WITH_PATCH_TEMPLATE_VAR: posixpath.join(
             STORAGE_HTTP_BASE,
             slave_info.files_location_withpatch.lstrip('gs://'))}
    )
    with open(os.path.join(output_html_dir, '%s.html' % slave_name),
              'w') as per_slave_html:
      per_slave_html.write(rendered)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--json_summaries_dir',
      help='Location of JSON summary files from all GCE slaves.')
  option_parser.add_option(
      '', '--output_html_dir',
      help='The absolute path of the HTML dir that will contain the results of'
           ' this script.')
  option_parser.add_option(
      '', '--absolute_url',
      help='Servers like Google Storage require an absolute url for links '
           'within the HTML output files.',
      default='')
  options, unused_args = option_parser.parse_args()
  if (not options.json_summaries_dir or not options.output_html_dir):
    option_parser.error(
        'Must specify json_summaries_dir and output_html_dir')

  OutputToHTML(
      CombineJsonSummaries(options.json_summaries_dir),
      options.output_html_dir,
      options.absolute_url)

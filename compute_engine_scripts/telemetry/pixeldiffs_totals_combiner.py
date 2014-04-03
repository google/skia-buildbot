#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to combine pixeldiff results and output summary in HTML."""


import optparse
import os
import posixpath
import subprocess

from django.template import loader

# Add the django settings file to DJANGO_SETTINGS_MODULE.
os.environ['DJANGO_SETTINGS_MODULE'] = 'csv-django-settings'


class SlaveInfo(object):
  """Container class that holds all slave data."""
  def __init__(self, slave_name, slave_diff_html_loc, failed_webpages_length,
               failed_to_load_urls_loc):
    self.slave_name = slave_name
    self.slave_diff_html_loc = slave_diff_html_loc
    self.failed_webpages_length = failed_webpages_length
    self.failed_to_load_urls_loc = failed_to_load_urls_loc


class PixelDiffsCombiner(object):
  """Class that combines pixeldiff results and outputs summary in HTML."""

  def __init__(self, pixeldiffs_gs_root, pixeldiffs_gs_http_path,
               requester_email, chromium_patch_link, blink_patch_link,
               skia_patch_link, output_html_dir):
    """Constructs a PixelDiffsCombiner instance."""
    self._pixeldiffs_gs_root = pixeldiffs_gs_root
    self._pixeldiffs_gs_http_path = pixeldiffs_gs_http_path
    self._requester_email = requester_email
    self._chromium_patch_link = chromium_patch_link
    self._blink_patch_link = blink_patch_link
    self._skia_patch_link = skia_patch_link
    self._output_html_dir = output_html_dir

  def _GetSlaveInfos(self):
    """Constructs a list of SlaveInfo objects."""

    slave_infos = []

    # Get list of slave directories from Google Storage.
    p = subprocess.Popen('gsutil ls -l %s' % self._pixeldiffs_gs_root,
                         shell=True, stdout=subprocess.PIPE)
    slave_dirs = p.stdout.read().split()

    # Loop through the slave directories and create SlaveInfo objects.
    for slave_dir in slave_dirs:
      slave_num = os.path.basename(slave_dir.rstrip('/'))
      p = subprocess.Popen(
          'gsutil cat %s' % posixpath.join(slave_dir, 'diff.html'),
          shell=True, stdout=subprocess.PIPE)
      diff_html_output = p.stdout.read()
      failed_webpages_length = diff_html_output.count('img src')

      # If the bad_urls.txt file exists then site it to SlaveInfo.
      p = subprocess.Popen(
          'gsutil ls -l %s' % posixpath.join(slave_dir, 'bad_urls.txt'),
          shell=True, stdout=subprocess.PIPE)
      if p.stdout.read():
        failed_to_load_urls_loc = posixpath.join(
            self._pixeldiffs_gs_http_path, slave_num, 'bad_urls.txt')
      else:
        failed_to_load_urls_loc = None
      slave_diff_html_loc = posixpath.join(
          self._pixeldiffs_gs_http_path, slave_num, 'diff.html')

      slave_infos.append(
          SlaveInfo(slave_num, slave_diff_html_loc, failed_webpages_length,
                    failed_to_load_urls_loc))

    return slave_infos

  def OutputResultsToHTML(self):
    """Outputs Pixeldiff results to HTML."""

    slave_infos = self._GetSlaveInfos()

    # Output the main totals HTML page.
    rendered = loader.render_to_string(
        'pixeldiff_totals.html',
        {'slave_infos': slave_infos,
         'requester_email': self._requester_email,
         'chromium_patch_link': self._chromium_patch_link,
         'blink_patch_link': self._blink_patch_link,
         'skia_patch_link': self._skia_patch_link})
    index_html = open(os.path.join(self._output_html_dir, 'index.html'), 'w')
    index_html.write(rendered)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--pixeldiffs_gs_root',
      help='The Google Storage root where all slaves posted results.')
  option_parser.add_option(
      '', '--pixeldiffs_gs_http_path',
      help='The Google Storage HTTP path where all slaves posted results.')
  option_parser.add_option(
      '', '--requester_email',
      help='Email address of the user who kicked off the run.')
  option_parser.add_option(
      '', '--chromium_patch_link',
      help='Link to the Chromium patch used for this run.')
  option_parser.add_option(
      '', '--blink_patch_link',
      help='Link to the Blink patch used for this run.')
  option_parser.add_option(
      '', '--skia_patch_link',
      help='Link to the Skia patch used for this run.')
  option_parser.add_option(
      '', '--output_html_dir',
      help='The directory where HTML files will be written to.')


  options, unused_args = option_parser.parse_args()
  if not (options.pixeldiffs_gs_root and options.pixeldiffs_gs_http_path
          and options.requester_email and options.chromium_patch_link
          and options.blink_patch_link and options.skia_patch_link
          and options.output_html_dir):
    option_parser.error('Must specify pixeldiffs_gs_root, '
                        'pixeldiffs_gs_http_path, requester_email, '
                        'chromium_patch_link, blink_patch_link, '
                        'skia_patch_link, output_html_dir')

  PixelDiffsCombiner(
      options.pixeldiffs_gs_root, options.pixeldiffs_gs_http_path,
      options.requester_email, options.chromium_patch_link,
      options.blink_patch_link, options.skia_patch_link,
      options.output_html_dir).OutputResultsToHTML()


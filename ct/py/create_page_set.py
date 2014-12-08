#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Creates a Python telemetry page_set from the specified webpages CSV.

This module does the following steps:
* Downloads a ZIP from http://s3.amazonaws.com/alexa-static/top-1m.csv.zip
* Unpacks it and reads its contents in memory.
* Writes out multiple Python page sets from the CSV file for the specified
  number of webpages.

Sample Usage:
  python create_page_set.py -s 1 -e 10000

Running the above command will create 10000 different page sets.
"""

__author__ = 'Ravi Mistry'

import optparse
import os
import urllib
import zipfile

from StringIO import StringIO


TOP1M_CSV_FILE_NAME = 'top-1m.csv'
TOP1M_CSV_ZIP_LOCATION = (
    'http://s3.amazonaws.com/alexa-static/%s.zip' % TOP1M_CSV_FILE_NAME)
ALEXA_PREFIX = 'alexa'


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '-s', '--start_number',
      help='Specifies where to start with when adding the top webpages to the '
           'page_set.',
      default='1')
  option_parser.add_option(
      '-e', '--end_number',
      help='Specifies where to end with when adding the top webpages to the '
           'page_set',
      default='10000')
  option_parser.add_option(
      '-c', '--csv_file',
      help='Location of a filtered alexa top 1M CSV file. Each row should '
           'have 3 entries, 1st will be rank, 2nd will be domain name and '
           'third will be the fully qualified url. If the third section is '
           'missing then a page_set for the URL will not be generated. If '
           'csv_file is not specified then this script downloads it from the '
           'internet.',
      default=None)
  option_parser.add_option(
      '-p', '--pagesets_type',
      help='The type of pagesets to create from the 1M list. Eg: All, '
           '100k, 10k, IndexSample10k, Mobile10k',
      default='All')
  option_parser.add_option(
      '-u', '--useragent_type',
      help='The type of user agent to use in the pagesets. Eg: desktop, '
           'mobile, tablet',
      default='desktop')
  option_parser.add_option(
      '-o', '--pagesets_output_dir',
      help='The directory generated pagesets will be outputted in.',
      default='page_sets')
  options, unused_args = option_parser.parse_args()

  # Validate arguments.
  if int(options.start_number) <= 0:
    raise Exception('The -s/--start_number must be greater than 0')
  if int(options.start_number) > int(options.end_number):
    raise Exception('The -s/--start_number must be less than or equal to '
                    '-e/--end_number')

  if options.csv_file:
    csv_contents = open(options.csv_file).readlines()
  else:
    # Download the zip file in member and extract its contents.
    usock = urllib.urlopen(TOP1M_CSV_ZIP_LOCATION)
    myzipfile = zipfile.ZipFile(StringIO(usock.read()))
    csv_contents = myzipfile.open(TOP1M_CSV_FILE_NAME).readlines()

  # Validate options.end_number.
  if int(options.end_number) > len(csv_contents):
    raise Exception('Please specify -e/--end_number less than or equal to %s' %
              len(csv_contents))

  websites = []
  for index in xrange(int(options.start_number) - 1, int(options.end_number)):
    line = csv_contents[index]
    website = line.strip().split(',')[1]
    if website.startswith('https://') or website.startswith('http://'):
      qualified_website = website
    else:
      qualified_website = 'http://www.%s' % website
    websites.append(qualified_website)

  archive_data_file = os.path.join(
      '/', 'b', 'storage', 'webpage_archives',
      options.pagesets_type,
      'alexa%s-%s.json' % (options.start_number, options.end_number))

  page_set_content = """
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
# pylint: disable=W0401,W0614

from telemetry.page import page as page_module
from telemetry.page import page_set as page_set_module


class TypicalAlexaPage(page_module.Page):

  def __init__(self, url, page_set):
    super(TypicalAlexaPage, self).__init__(url=url, page_set=page_set)
    self.user_agent_type = '%(user_agent)s'
    self.archive_data_file = '%(archive_data_file)s'

  def RunSmoothness(self, action_runner):
    action_runner.ScrollElement()

  def RunRepaint(self, action_runner):
    action_runner.RepaintContinuously(seconds=5)


class Alexa%(start)s_%(end)sPageSet(page_set_module.PageSet):

  def __init__(self):
    super(Alexa%(start)s_%(end)sPageSet, self).__init__(
      user_agent_type='%(user_agent)s',
      archive_data_file='%(archive_data_file)s')

    urls_list = %(urls_list)s

    for url in urls_list:
      self.AddPage(TypicalAlexaPage(url, self))
""" % {
      "user_agent": options.useragent_type,
      "archive_data_file": archive_data_file,
      "start": options.start_number,
      "end": options.end_number,
      "urls_list": str(websites),
  }

  # Output the pageset to a file.
  with open(os.path.join(options.pagesets_output_dir, 'alexa%s_%s.py' % (
                options.start_number, options.end_number)),
            'w') as outfile:
    outfile.write(page_set_content)


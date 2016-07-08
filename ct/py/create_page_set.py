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
  python create_page_set.py -s 1

Running the above command will create a page set with the webpage in 1st
position in the CSV.
"""

__author__ = 'Ravi Mistry'

import json
import optparse
import os
import urllib
import zipfile

from StringIO import StringIO


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '-s', '--position',
      help='Specifies the position of the webpage in the CSV which will be '
           'added to the page_set.',
      default='1')
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
  if int(options.position) <= 0:
    raise Exception('The -s/--position must be greater than 0')

  with open(options.csv_file) as fp:
    for i, l in enumerate(fp):
      if i == (int(options.position)-1):
        line = l
        break

  websites = []
  website = line.strip().split(',')[1]
  if 'PDF' in options.pagesets_type:
    # PDF urls do not need any additional prefixes.
    qualified_website = website
  elif website.startswith('https://') or website.startswith('http://'):
    qualified_website = website
  else:
    qualified_website = 'http://www.%s' % website
  websites.append(qualified_website)

  archive_data_file = os.path.join(
      '/', 'b', 'storage', 'webpage_archives',
      options.pagesets_type,
      '%s.json' % options.position)

  page_set_content = {
    'user_agent': options.useragent_type,
    'archive_data_file': archive_data_file,
    'urls_list': ','.join(websites),
  }

  # Output the pageset to a file.
  with open(os.path.join(options.pagesets_output_dir, '%s.py' % (
                options.position)),
            'w') as outfile:
    json.dump(page_set_content, outfile)

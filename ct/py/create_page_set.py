#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Creates a Python telemetry page_set from the specified webpages CSV.

This module writes out multiple Python page sets from the specified CSV file.

Sample Usage:
  python create_page_set.py -s 1 -e 10 -c /tmp/test.csv

Running the above command will create 10 page sets containing webpages from the
1st to 10th position in the CSV.
"""

__author__ = 'Ravi Mistry'

import json
import optparse
import os
import urllib
import zipfile


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '-s', '--position',
      help='Specifies the start position of the webpages in the CSV which will '
           'be created as pagesets.',
      default='1')
  option_parser.add_option(
      '-e', '--end',
      help='Specifies the end position of the webpages in the CSV which will '
           'be created as pagesets. This is inclusive.',
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
  if int(options.end) < int(options.position):
    raise Exception('The -e/--end must be >= than -s/--position')

  start_index = int(options.position) - 1
  with open(options.csv_file) as fp:
    for i, line in enumerate(fp):
      if i < start_index:
        continue
      elif i >= int(options.end):
        break
      else:
        websites = []
        website = line.strip().split(',')[1]
        if website.startswith('https://') or website.startswith('http://'):
          qualified_website = website
        elif len(website.split('.')) > 2:
          qualified_website = 'http://%s' % website
        else:
          qualified_website = 'http://www.%s' % website
        websites.append(qualified_website)

        archive_data_file = os.path.join(
            '/', 'b', 'storage', 'webpage_archives',
            options.pagesets_type,
            '%s.json' % (i+1))

        page_set_content = {
          'user_agent': options.useragent_type,
          'archive_data_file': archive_data_file,
          'urls_list': ','.join(websites),
        }

        # Output the pageset to a file.
        parent_dir = os.path.join(options.pagesets_output_dir, str(i+1))
        os.mkdir(parent_dir)
        with open(os.path.join(parent_dir, '%s.py' % (i+1)), 'w') as outfile:
          json.dump(page_set_content, outfile)

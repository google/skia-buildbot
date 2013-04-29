#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Downloads CSV of top 1M webpages and creates a JSON telemetry page_set.

This module does the following steps:
* Downloads a ZIP from http://s3.amazonaws.com/alexa-static/top-1m.csv.zip
* Unpacks it and reads its contents in memory.
* Writes out multiple JSON page sets from the CSV file for the specified number
  of webpages.

Note: Blacklisted webpages (broken or spyware filled webpages) will not be added
to the outputted JSON page_set. If you request 100 webpages and 5 of them are
blacklisted then the page_set will only contain 95 webpages.

Sample Usage:
  python create_page_set.py -n 10000

Running the above command will create 10000 different page sets.
The outputted page sets are intended to be used by the webpages_playback.py
script.
Sample usage of the webpages_playback.py script with the outputted page sets:
  python webpages_playback.py --record=True /
  --page_sets=../../tools/page_sets/*.json /
  --do_not_upload_to_gs=True --output_dir=/network/accessible/moint/point/
"""

__author__ = 'Ravi Mistry'

import getpass
import json
import optparse
import os
import urllib
import zipfile

from datetime import datetime
from StringIO import StringIO


TOP1M_CSV_FILE_NAME = 'top-1m.csv'
TOP1M_CSV_ZIP_LOCATION = (
    'http://s3.amazonaws.com/alexa-static/%s.zip' % TOP1M_CSV_FILE_NAME)


# List of broken or spyware filled webpages.
blacklisted_webpages = [
    'm5zn.com',
    'yieldmanager.com',
]

# Webpages that need more loading time.
slow_webpages = {
    'gavick.com': 15.0,
    'rapidgator.net': 15.0,
    'xhamster.com': 15.0,
}

# Webpages that need to be mapped to another name to work.
mapped_webpages = {
    'justhost.com': 'www.justhost.com',
}


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '-n', '--number',
      help='Specifies how many top webpages should be added to the page_set.',
      default='10000')
  options, unused_args = option_parser.parse_args()

  # Download the zip file in member and extract its contents.
  usock = urllib.urlopen(TOP1M_CSV_ZIP_LOCATION)
  myzipfile = zipfile.ZipFile(StringIO(usock.read()))
  csv_contents = myzipfile.open(TOP1M_CSV_FILE_NAME).readlines()

  # Validate options.number
  if int(options.number) > len(csv_contents):
    raise Exception('Please specify -n/--number less than or equal to %s' %
              len(csv_contents))

  # Populate the JSON dictionary.
  json_dict = {
      '_comment': 'Generated on %s by %s using create_page_set.py' % (
          datetime.now(), getpass.getuser()),
      'description': 'Top %s Alexa global.' % options.number,
  }
  for index in xrange(0, int(options.number)):
    line = csv_contents[index]
    (unused_number, website) = line.strip().split(',')
    if website in blacklisted_webpages:
      continue
    website_filename = 'alexa%s_%s_desktop' % (
        index + 1, website.replace('.', '-').replace('/', '-'))
    website_specific_info = {
        'archive_path': os.path.join(os.pardir, os.pardir, 'slave',
                                     'skia_slave_scripts', 'page_sets', 'data',
                                     '%s.wpr' % website_filename),
        'pages': [{
            # fully qualified CSV websites.
            'url': 'http://%s' % mapped_webpages.get(website, website),
            'wait_time_after_navigate': slow_webpages.get(website, 5.0),
            'why': '#%s in Alexa global.' % (index + 1)
         }]
    }
    json_dict.update(website_specific_info)

    # Output the JSON dictionary to a file.
    with open(os.path.join('page_sets', '%s.json' % website_filename),
              'w') as outfile:
      json.dump(json_dict, outfile, indent=4)


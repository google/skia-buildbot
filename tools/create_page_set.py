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

Note: Blacklisted webpages will not be added to the outputted JSON page_set. If
you request 100 webpages and 5 of them are blacklisted then the page_set will
only contain 95 webpages.

Sample Usage:
  python create_page_set.py -s 1 -e 10000

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
ALEXA_PREFIX = 'alexa'

# Webpages that need to be mapped to another name to work.
mapped_webpages = {
    'justhost.com': 'www.justhost.com',
}


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
      '-b', '--blacklist',
      help='Location of a black_list file which specifies which webpages '
           'should not be converted into page_sets.',
      default='blacklist')
  option_parser.add_option(
      '-c', '--csv_file',
      help='Location of the alexa top 1M CSV file. This script downloads it '
           'from the internet if it is not specified.',
      default=None)
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

  # Populate the JSON dictionary.
  pages = []
  json_dict = {
      '_comment': 'Generated on %s by %s using create_page_set.py' % (
          datetime.now(), getpass.getuser()),
      'description': 'Top %s-%s Alexa global.' % (options.start_number,
                                                  options.end_number),
      'archive_data_file': os.path.join(
          '/', 'home', 'default', 'storage', 'webpages_archive',
          'alexa%s-%s.json' % (options.start_number, options.end_number)),
      'pages': pages,
      'smoothness': { 'action': 'scroll'},
      'user_agent_type': 'desktop',
  }

  blacklisted_webpages = (open(options.blacklist).readlines()
                          if options.blacklist else [])

  for index in xrange(int(options.start_number) - 1, int(options.end_number)):
    line = csv_contents[index]
    (unused_number, website) = line.strip().split(',')
    website_filename = '%s%s_%s_desktop' % (
        ALEXA_PREFIX, index + 1, website.replace('.', '-').replace('/', '-'))

    skip_webpage = False
    for blacklisted_webpage in blacklisted_webpages:
      if blacklisted_webpage.rstrip() in website_filename:
        skip_webpage = True
        break
    if skip_webpage:
      print 'Skipping %s because it is in the provided blacklist file!' % (
          website_filename)
      continue
    pages.append({
        # fully qualified CSV websites.
        'url': 'http://%s' % mapped_webpages.get(website, website),
        'why': '#%s in Alexa global.' % (index + 1)
        })

    # Output the JSON dictionary to a file.
    try:
      with open(os.path.join('page_sets', 'alexa%s-%s.json' % (
                    options.start_number, options.end_number)),
                'w') as outfile:
        json.dump(json_dict, outfile, indent=4)
    except Exception, e:
      print 'Skipping %s because it failed with Exception: %s' % (
          website_filename, e)


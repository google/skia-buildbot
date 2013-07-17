#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to create the ranking CSV file from pagesets."""


import glob
import optparse
import os
import re
import sys


class CsvRanker(object):

  def __init__(self, slave_num):
    self._slave_num = slave_num

  def Create(self):
    page_sets = glob.glob('/home/default/storage/page_sets/*.json')
    rank_and_page = []
    for page_set in page_sets:
      content = open(page_set).read()
      # Remove newlines.
      content = content.replace('\n', '')
      # Parse out the url and the rank.
      match_obj = re.match(
          '.*\"url\"\: \"(.*)\"\,.*\"why\": \"#(.*) in Alexa.*', content)
      url = match_obj.group(1).split('://')[1]
      rank = int(match_obj.group(2))
      rank_and_page.append((rank, url))
    rank_and_page.sort()

    # Write the rank and page to a CSV file and then store in google storage.
    output_filename = '/tmp/top-1m-%s.csv' % self._slave_num
    output_csv = open(output_filename, 'w')
    for rank, page in rank_and_page:
      output_csv.write('%s,%s\n' % (rank, page))
    output_csv.close()

    # Copy to Google Storage.
    os.system('gsutil cp %s gs://chromium-skia-gm/telemetry/csv/slave%s/' % (
        output_filename, self._slave_num))

    # Delete the local CSV file.
    os.remove(output_filename)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--slave_num',
      help='The number of the current telemetry slave')
  options, unused_args = option_parser.parse_args()

  if not options.slave_num:
    option_parser.error('Must specify the slave number')

  sys.exit(CsvRanker(options.slave_num).Create())


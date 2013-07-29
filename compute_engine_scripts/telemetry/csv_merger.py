#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to merge many CSV files into a single file."""

import csv
import glob
import optparse
import os
import sys


class CsvMerger(object):
  """Class that merges many CSV files into a single file."""

  def __init__(self, csv_dir, output_csv_name):
    """Constructs a CsvMerge instance."""
    self._input_csv_files = sorted([
        os.path.join(csv_dir, f) for f in
        glob.glob(os.path.join(csv_dir, '*.csv'))
        if os.path.getsize(os.path.join(csv_dir, f))])
    self._output_csv_name = os.path.join(csv_dir, output_csv_name)

  def _GetFieldNames(self):
    field_names = set()
    for csv_file in self._input_csv_files:
      field_names.update(csv.DictReader(open(csv_file, 'r')).fieldnames)
    return field_names

  def Merge(self):
    """Method that does the CSV merging."""
    field_names = self._GetFieldNames()
    print 'Merging %d csv files into %d columns' % (len(self._input_csv_files),
                                                    len(field_names))

    dict_writer = csv.DictWriter(open(self._output_csv_name, 'w'), field_names)
    dict_writer.writeheader()

    total_rows = 0

    for csv_file in self._input_csv_files:
      print 'Merging %s' % csv_file

      dict_reader = csv.DictReader(open(csv_file, 'r'))
      for row in dict_reader:
        dict_writer.writerow(row)
        total_rows += 1

    print 'Successfully merged %d rows' % total_rows


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--csv_dir',
      help='Directory that contains the csv files to be merged. This directory'
           ' will also contain the merged CSV.')
  option_parser.add_option(
      '', '--output_csv_name',
      help='The name of the resultant merged CSV. It will be outputted to the '
           '--csv_dir')
  options, unused_args = option_parser.parse_args()
  if not options.csv_dir or not options.output_csv_name:
    option_parser.error('Must specify bot csv_dir and output_csv_name')

  sys.exit(CsvMerger(options.csv_dir, options.output_csv_name).Merge())

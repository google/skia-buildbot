#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to merge many CSV files into a single file.

If there are multiple CSV files with the same TELEMETRY_PAGE_NAME_KEY then the
median of all values is stored in the resultant CSV file.
"""


import csv
import glob
import optparse
import os
import sys


TELEMETRY_PAGE_NAME_KEY = 'page_name'


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

  def _GetMedian(self, l):
    """Returns the median value from the specified list."""
    l.sort()
    length = len(l)
    if not length % 2:
      return (l[(length/2) - 1] + l[length/2]) / 2
    else:
      return l[length/2]

  def _GetRowWithMedianValues(self, rows):
    """Parses the specified rows and returns a single row with median values."""
    fieldname_to_values = {}
    for row in rows:
      for fieldname in row:
        if fieldname == TELEMETRY_PAGE_NAME_KEY:
          fieldname_to_values[fieldname] = row[fieldname]
          continue
        try:
          value = float(row[fieldname])
        except ValueError:
          # We expected only floats, cannot compare strings. Skip this field.
          continue
        if fieldname in fieldname_to_values:
          fieldname_to_values[fieldname].append(value)
        else:
          fieldname_to_values[fieldname] = [value]

    median_row = {}
    for fieldname, values in fieldname_to_values.items():
      if fieldname == TELEMETRY_PAGE_NAME_KEY:
        median_row[fieldname] = values
        continue
      median_row[fieldname] = self._GetMedian(values)

    print
    print 'For rows: %s' % rows
    print 'Median row is %s' % median_row
    print
    return median_row

  def Merge(self):
    """Method that does the CSV merging."""
    field_names = self._GetFieldNames()
    print 'Merging %d csv files into %d columns' % (len(self._input_csv_files),
                                                    len(field_names))

    # List that will contain all rows read from the CSV files. It will also
    # combine all rows found with the same TELEMETRY_PAGE_NAME_KEY into one
    # with median values.
    csv_rows = []

    # Dictionary containing all the encountered page names. If a page name that
    # is already in the dictionary is encountered then the median of its
    # values is used.
    page_names_to_rows = {}

    for csv_file in self._input_csv_files:
      dict_reader = csv.DictReader(open(csv_file, 'r'))
      for row in dict_reader:
        if TELEMETRY_PAGE_NAME_KEY in row:
          # Add rows found with 'page_name' to a different dictionary for
          # processing.
          if row[TELEMETRY_PAGE_NAME_KEY] in page_names_to_rows:
            page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]].append(row)
          else:
            page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]] = [row]
        else:
          # Add rows found without TELEMETRY_PAGE_NAME_KEY to the final list of
          # rows, they require no further processing.
          csv_rows.append(row)

    if page_names_to_rows:
      for page_name in page_names_to_rows:
        rows = page_names_to_rows[page_name]
        median_row = self._GetRowWithMedianValues(rows)
        # Add a single row that contains median values from all rows with the
        # same TELEMETRY_PAGE_NAME_KEY.
        csv_rows.append(median_row)

    # Write all rows in csv_rows to the specified output CSV.
    dict_writer = csv.DictWriter(open(self._output_csv_name, 'w'), field_names)
    dict_writer.writeheader()
    total_rows = 0
    for row in csv_rows:
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
    option_parser.error('Must specify both csv_dir and output_csv_name')

  sys.exit(CsvMerger(options.csv_dir, options.output_csv_name).Merge())

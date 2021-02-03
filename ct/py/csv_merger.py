#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to merge many CSV files into a single file.

If there are multiple CSV files with the same TELEMETRY_PAGE_NAME_KEY then the
smallest of all values is stored in the resultant CSV file.
"""


import csv
import glob
import optparse
import os
import sys


TELEMETRY_PAGE_NAME_KEY = 'page_name'


class CsvMerger(object):
  """Class that merges many CSV files into a single file."""

  def __init__(self, csv_dir, output_csv_name, handle_strings):
    """Constructs a CsvMerge instance."""
    self._input_csv_files = sorted([
        os.path.join(csv_dir, f) for f in
        glob.glob(os.path.join(csv_dir, '*.csv'))
        if os.path.getsize(os.path.join(csv_dir, f))])
    self._output_csv_name = os.path.join(csv_dir, output_csv_name)
    self._handle_strings = handle_strings

  def _GetFieldNames(self):
    field_names = set()
    for csv_file in self._input_csv_files:
      with open(csv_file, 'rb') as f:
        field_names.update(csv.DictReader(f).fieldnames)
    return field_names

  def _GetSmallest(self, l):
    """Returns the smallest value from the specified list."""
    l.sort()
    return l[0]

  def _GetRowWithSmallestValues(self, rows):
    """Parses the specified rows and returns a row with the smallest values."""
    fieldname_to_values = {}
    for row in rows:
      for fieldname in row:
        if fieldname == TELEMETRY_PAGE_NAME_KEY:
          fieldname_to_values[fieldname] = row[fieldname]
          continue
        try:
          value = float(row[fieldname])
        except (ValueError, TypeError):
          if row[fieldname] and self._handle_strings:
            # Use the original value and proceed.
            value = row[fieldname]
          else:
            # We expected only floats, cannot compare strings. Skip this field.
            continue
        if fieldname in fieldname_to_values:
          fieldname_to_values[fieldname].append(value)
        else:
          fieldname_to_values[fieldname] = [value]

    smallest_row = {}
    for fieldname, values in fieldname_to_values.items():
      if fieldname == TELEMETRY_PAGE_NAME_KEY:
        smallest_row[fieldname] = values
        continue
      try:
        smallest_row[fieldname] = self._GetSmallest(values)
      except (ValueError, TypeError):
        smallest_row[fieldname] = ','.join(values)

    # print
    # print 'For rows: %s' % rows
    # print 'Smallest row is %s' % smallest_row
    # print
    return smallest_row

  def Merge(self):
    """Method that does the CSV merging."""
    field_names = self._GetFieldNames()
    print 'Merging %d csv files into %d columns' % (len(self._input_csv_files),
                                                    len(field_names))

    # List that will contain all rows read from the CSV files. It will also
    # combine all rows found with the same TELEMETRY_PAGE_NAME_KEY into one
    # with smallest values.
    csv_rows = []

    # Dictionary containing all the encountered page names. If a page name that
    # is already in the dictionary is encountered then the smallest of its
    # values is used.
    page_names_to_rows = {}

    for csv_file in self._input_csv_files:
      with open(csv_file, 'rb') as f:
        dict_reader = csv.DictReader(f)
        for row in dict_reader:
          if TELEMETRY_PAGE_NAME_KEY in row:
            # Add rows found with 'page_name' to a different dictionary for
            # processing.
            if row[TELEMETRY_PAGE_NAME_KEY] in page_names_to_rows:
              page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]].append(row)
            else:
              page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]] = [row]
          else:
            # Add rows found without TELEMETRY_PAGE_NAME_KEY to the final list
            # of rows, they require no further processing.
            csv_rows.append(row)

    if page_names_to_rows:
      for page_name in page_names_to_rows:
        rows = page_names_to_rows[page_name]
        smallest_row = self._GetRowWithSmallestValues(rows)
        # Add a single row that contains smallest values from all rows with the
        # same TELEMETRY_PAGE_NAME_KEY.
        csv_rows.append(smallest_row)

    # Write all rows in csv_rows to the specified output CSV.
    with open(self._output_csv_name, 'wb') as f:
      dict_writer = csv.DictWriter(f, field_names)
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
  option_parser.add_option(
      '', '--handle_strings', action="store_true", default=False,
      help='If this option is False then rows with string values are dropped')
  options, unused_args = option_parser.parse_args()
  if not options.csv_dir or not options.output_csv_name:
    option_parser.error('Must specify both csv_dir and output_csv_name')

  sys.exit(CsvMerger(options.csv_dir, options.output_csv_name,
                     options.handle_strings).Merge())

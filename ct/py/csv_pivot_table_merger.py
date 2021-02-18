#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to merge many CSV files into a single file.

If there are multiple CSV files with the same TELEMETRY_PAGE_NAME_KEY then the
avg of all values is stored in the resultant CSV file.
"""


import csv
import glob
import optparse
import os
import sys


TELEMETRY_PAGE_NAME_KEY = 'stories'
TELEMETRY_FIELD_NAME_KEY = 'name'
TELEMETRY_FIELD_UNITS_KEY = 'unit'
# Special handling for traceURLs. See skbug.com/7212.
TELEMETRY_TRACE_URLS_KEY = 'traceUrls'

OUTPUT_PAGE_NAME_KEY = 'page_name'


class CsvMerger(object):
  """Class that merges many CSV files into a single file."""

  def __init__(self, csv_dir, output_csv_name, value_column_name,
               handle_strings):
    """Constructs a CsvMerge instance."""
    self._input_csv_files = sorted([
        os.path.join(csv_dir, f) for f in
        glob.glob(os.path.join(csv_dir, '*.csv'))
        if os.path.getsize(os.path.join(csv_dir, f))])
    self._output_csv_name = os.path.join(csv_dir, output_csv_name)
    self._value_column_name = value_column_name
    self._handle_strings = handle_strings

  def _GetFieldNameFromRow(self, row):
    fieldname = row[TELEMETRY_FIELD_NAME_KEY]
    units = row[TELEMETRY_FIELD_UNITS_KEY]
    if units:
      fieldname = '%s (%s)' % (fieldname, units)
    return fieldname

  def _GetFieldNames(self):
    field_names = set()
    for csv_file in self._input_csv_files:
      with open(csv_file, 'rb') as f:
        dict_reader = csv.DictReader(f)
        for row in dict_reader:
          field_name = self._GetFieldNameFromRow(row)
          field_names.add(field_name)
    # We use 'page_name' in the output CSV to ID the webpage.
    field_names.add(OUTPUT_PAGE_NAME_KEY);
    field_names.add(TELEMETRY_TRACE_URLS_KEY);
    return field_names

  def _GetSmallest(self, l):
    """Returns the smallest value from the specified list."""
    l.sort()
    return l[0]

  def _GetAvg(self, l):
    """Returns the avg value from the specified list."""
    avg = 0
    for v in l:
      avg += v
    return avg/len(l)

  def _GetTraceURLVal(self, values):
    if not values:
      return ''
    # Deduplicate and maintain the order of items in the list.
    seen = set()
    return ','.join([x for x in values if not (x in seen or seen.add(x))])

  def _GetRowWithAvgValues(self, rows):
    """Parses the specified rows and returns a row with the avg values."""
    fieldname_to_values = {}
    for row in rows:
      page_name = row[TELEMETRY_PAGE_NAME_KEY]
      value = row[self._value_column_name]
      fieldname = self._GetFieldNameFromRow(row)
      fieldname_to_values[OUTPUT_PAGE_NAME_KEY] = page_name

      # For some reason traceURLs have carriage returns on Windows. We need to
      # check for them and strip them out. See skbug.com/10590 for context.
      traceURL = (row.get(TELEMETRY_TRACE_URLS_KEY) or
                  row.get(TELEMETRY_TRACE_URLS_KEY + '\r'))
      if traceURL:
        traceURL = traceURL.rstrip('\r')
        if TELEMETRY_TRACE_URLS_KEY in fieldname_to_values:
          fieldname_to_values[TELEMETRY_TRACE_URLS_KEY].append(traceURL)
        else:
          fieldname_to_values[TELEMETRY_TRACE_URLS_KEY] = [traceURL]

      try:
        value = float(value)
      except (ValueError, TypeError):
        if value and self._handle_strings:
          # Keep the original value and proceed.
          pass
        else:
          # We expected only floats, cannot compare strings. Skip this row.
          continue
      if fieldname in fieldname_to_values:
        fieldname_to_values[fieldname].append(value)
      else:
        fieldname_to_values[fieldname] = [value]

    avg_row = {}
    for fieldname, values in fieldname_to_values.items():
      if fieldname == OUTPUT_PAGE_NAME_KEY:
        avg_row[fieldname] = values
        continue
      elif fieldname == TELEMETRY_TRACE_URLS_KEY:
        avg_row[fieldname] = self._GetTraceURLVal(values)
        continue
      try:
        avg_row[fieldname] = self._GetAvg(values)
      except (ValueError, TypeError):
        avg_row[fieldname] = ','.join(values)

    print
    print 'For rows: %s' % rows
    print 'Avg row is %s' % avg_row
    print
    return avg_row

  def Merge(self):
    """Method that does the CSV merging."""
    field_names = self._GetFieldNames()
    print 'Merging %d csv files into %d columns' % (len(self._input_csv_files),
                                                    len(field_names))

    # List that will contain all rows read from the CSV files. It will also
    # combine all rows found with the same TELEMETRY_PAGE_NAME_KEY into one
    # with avg values.
    csv_rows = []

    # Dictionary containing all the encountered page names. If a page name that
    # is already in the dictionary is encountered then the avg of its
    # values is used.
    page_names_to_rows = {}

    for csv_file in self._input_csv_files:
      with open(csv_file, 'rb') as f:
        dict_reader = csv.DictReader(f)
        for row in dict_reader:
          # Ensure that the row contains page name and that is not empty.
          if TELEMETRY_PAGE_NAME_KEY in row and row[TELEMETRY_PAGE_NAME_KEY]:
            if row[TELEMETRY_PAGE_NAME_KEY] in page_names_to_rows:
              page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]].append(row)
            else:
              page_names_to_rows[row[TELEMETRY_PAGE_NAME_KEY]] = [row]

    if page_names_to_rows:
      for page_name in page_names_to_rows:
        rows = page_names_to_rows[page_name]
        avg_row = self._GetRowWithAvgValues(rows)
        # Add a single row that contains avg values from all rows with the
        # same TELEMETRY_PAGE_NAME_KEY.
        csv_rows.append(avg_row)

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
      '', '--value_column_name', default='avg',
      help='Which columns entry to use as field values when combining CSVs.')
  option_parser.add_option(
      '', '--handle_strings', action="store_true", default=False,
      help='If this option is False then rows with string values are dropped')
  options, unused_args = option_parser.parse_args()
  if not options.csv_dir or not options.output_csv_name:
    option_parser.error('Must specify both csv_dir and output_csv_name')

  sys.exit(CsvMerger(options.csv_dir, options.output_csv_name,
                     options.value_column_name,
                     options.handle_strings).Merge())

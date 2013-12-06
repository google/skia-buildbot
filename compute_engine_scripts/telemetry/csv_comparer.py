#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to compare two CSV files and output HTML results."""


import csv
import optparse
import os
import sys

from django.template import loader

# Add the django settings file to DJANGO_SETTINGS_MODULE.
os.environ['DJANGO_SETTINGS_MODULE'] = 'csv-django-settings'


def _GetPercentageDiff(value1, value2):
  """Returns the percentage difference between the specified values."""
  difference = value2 - value1
  avg = (value2 + value1)/2
  return 0 if avg == 0 else difference/avg * 100


class PageValues(object):
  """Container class to hold the values of a page name."""
  def __init__(self, page_name, value1, value2, perc_diff):
    self.page_name = page_name
    self.value1 = value1
    self.value2 = value2
    self.perc_diff = perc_diff


class FieldNameValues(object):
  """Container class to hold the values of a field name."""
  def __init__(self, value1, value2, perc_diff):
    self.value1 = value1
    self.value2 = value2
    self.perc_diff = perc_diff


class CsvComparer(object):
  """Class that compares two telemetry CSV files and outputs HTML results."""

  def __init__(self, csv_file1, csv_file2, output_html_dir,
               variance_threshold, absolute_url, min_pages_in_each_field,
               discard_outliers):
    """Constructs a CsvComparer instance."""
    self._csv_file1 = csv_file1
    self._csv_file2 = csv_file2
    self._output_html_dir = output_html_dir
    self._variance_threshold = float(variance_threshold)
    self._absolute_url = absolute_url
    self._min_pages_in_each_field = min_pages_in_each_field
    self._discard_outliers = float(discard_outliers)

  def _IsPercDiffAboveThreshold(self, perc_diff):
    """Compares the specified diff to the variance threshold.

    Returns True if the difference is above the variance threshold.
    """
    return abs(perc_diff) > self._variance_threshold

  def Compare(self):
    """Method that does the CSV comparision."""

    # Do one pass of all the page_names in the 1st CSV and store them.
    # The purpose of this is that when we walk through the 2nd CSV we will know
    # Whether the same page exists in the 1st CSV (the pages are ordered the
    # same way in both files but some could be missing from each file).
    csv1_page_names = {}
    csv1_reader = csv.DictReader(open(self._csv_file1, 'r'))
    for row in csv1_reader:
      csv1_page_names[row['page_name']] = 1

    csv1_reader = csv.DictReader(open(self._csv_file1, 'r'))
    csv2_reader = csv.DictReader(open(self._csv_file2, 'r'))

    # Dictionary that holds the fieldname to the ongoing total on both CSVs.
    fieldnames_to_totals = {}
    # Map of a fieldname to list of tuples containing (page_name, csv_value1,
    # csv_value2, percentage_difference).
    fieldnames_to_page_values = {}

    # Now walk through both CSV files with a pointer at each one and collect
    # the value totals.
    for csv2_row in csv2_reader:
      # Make sure the CSV2 page_name existings in CSV1 else skip it (move CSV2
      # pointer down).
      page_name2 = csv2_row['page_name']
      if not csv1_page_names.has_key(page_name2):
        continue
      # Reach the right page_name in CSV1 (move CSV1 pointer down).
      try:
        csv1_row = csv1_reader.next()
        while csv1_row['page_name'] != page_name2:
          csv1_row = csv1_reader.next()
      except StopIteration:
        # Reached the end of CSV1, break out of the row loop.
        break

      # Store values for all fieldnames (except page_name).
      for fieldname in csv2_reader.fieldnames:
        if fieldname != 'page_name' and csv1_row.has_key(fieldname):
          if csv1_row[fieldname] == '' or csv2_row[fieldname] == '':
            # TODO(rmistry): Check with tonyg about what the algorithm should
            # be doing when one CSV has an empty value and the other does not.
            continue
          try:
            csv1_value = float(csv1_row.get(fieldname))
            csv2_value = float(csv2_row.get(fieldname))
          except ValueError:
            # We expected only floats, cannot compare strings. Skip this field.
            continue
          if csv1_value < 0 or csv2_value < 0:
            # If either value is less than 0 then something went wrong, skip it.
            continue

          # Update the total in the dict.
          fieldname_values = fieldnames_to_totals.get(
              fieldname, FieldNameValues(0, 0, 0))
          fieldname_values.value1 += csv1_value
          fieldname_values.value2 += csv2_value
          fieldnames_to_totals[fieldname] = fieldname_values

          perc_diff = _GetPercentageDiff(csv1_value, csv2_value)
          if self._IsPercDiffAboveThreshold(perc_diff):
            # Add this page only if its diff is above the threshold.
            l = fieldnames_to_page_values.get(fieldname, [])
            l.append(PageValues(page_name2, csv1_value, csv2_value, perc_diff))
            fieldnames_to_page_values[fieldname] = l

    # Calculate and add the percentage differences for each fieldname.
    # The fieldnames_to_totals dict is modified in the below loop to remove
    # entries which are below the threshold .
    for fieldname, fieldname_values in fieldnames_to_totals.items():
      if not fieldnames_to_page_values.has_key(fieldname):
        del fieldnames_to_totals[fieldname]
        continue

      page_values = fieldnames_to_page_values[fieldname]
      # Sort page values by the percentage difference.
      page_values.sort(key=lambda page_value: page_value.perc_diff,
                       reverse=True)

      if self._discard_outliers:
        # Lose the top X% and the bottom X%
        outliers_num = int(len(page_values) * self._discard_outliers/100)
        top_outliers = page_values[0:outliers_num]
        bottom_outliers = page_values[-outliers_num:]
        # Discard top and bottom outliers.
        fieldnames_to_page_values[fieldname] = (
            page_values[outliers_num:-outliers_num])
        # Remove discarded values from the running totals.
        for discarded_page in top_outliers + bottom_outliers:
          fieldname_values.value1 -= discarded_page.value1
          fieldname_values.value2 -= discarded_page.value2

      perc_diff = _GetPercentageDiff(fieldname_values.value1,
                                     fieldname_values.value2)
      if self._IsPercDiffAboveThreshold(perc_diff):
        if (len(fieldnames_to_page_values[fieldname]) <
            self._min_pages_in_each_field):
          # This field does not have enough webpages, delete it from both maps.
          del fieldnames_to_page_values[fieldname]
          del fieldnames_to_totals[fieldname]
          continue
        fieldname_values.perc_diff = perc_diff
      else:
        # Only store fieldnames that are above the variance threshold.
        del fieldnames_to_totals[fieldname]

    # Delete keys in fieldnames_to_page_values that are not in
    # fieldnames_to_totals because those are the only ones we want to
    # display.
    fieldnames_to_page_values = dict(
        (k,v) for k,v in fieldnames_to_page_values.iteritems()
        if k in fieldnames_to_totals)

    # Both maps should end up with the same number of keys.
    assert set(fieldnames_to_page_values.keys()) == set(
        fieldnames_to_totals.keys())

    # Done processing. Output the HTML.
    self.OutputToHTML(fieldnames_to_totals, fieldnames_to_page_values,
                      self._output_html_dir)

  def OutputToHTML(self, fieldnames_to_totals, fieldnames_to_page_values,
                   html_dir):
    # Output the main totals HTML page.
    sorted_fieldnames_totals_items = sorted(
        fieldnames_to_totals.items(), key=lambda tuple: tuple[1].perc_diff,
        reverse=True)
    rendered = loader.render_to_string(
        'csv_totals.html',
        {'sorted_fieldnames_totals_items': sorted_fieldnames_totals_items,
         'threshold': self._variance_threshold,
         'discard_outliers': self._discard_outliers,
         'min_webpages': self._min_pages_in_each_field,
         'absolute_url': self._absolute_url})
    index_html = open(os.path.join(self._output_html_dir, 'index.html'), 'w')
    index_html.write(rendered)

    # Output the different per-fieldname HTML pages.
    fieldname_count = 0
    # pylint: disable=W0612
    for fieldname, unused_values in sorted_fieldnames_totals_items:
      fieldname_count += 1
      page_values = fieldnames_to_page_values[fieldname]
      rendered = loader.render_to_string(
          'csv_per_page.html',
          {'fieldname': fieldname,
           'page_values': page_values,
           'absolute_url': self._absolute_url})
      fieldname_html = open(
          os.path.join(self._output_html_dir,
          'fieldname%s.html' % fieldname_count), 'w')
      fieldname_html.write(rendered)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--csv_file1',
      help='The absolute path to the first CSV file.')
  option_parser.add_option(
      '', '--csv_file2',
      help='The absolute path to the second CSV file.')
  option_parser.add_option(
      '', '--output_html_dir',
      help='The absolute path of the HTML dir that will contain the results of'
           ' the comparision CSV.')
  option_parser.add_option(
      '', '--variance_threshold',
      help='The allowable variance in percentage between total values for each '
           'field for the two CSVs.')
  option_parser.add_option(
      '', '--absolute_url',
      help='Servers like Google Storage require an absolute url for links '
           'within the HTML output files.',
      default='')
  option_parser.add_option(
      '', '--min_pages_in_each_field',
      help='The min number of pages that must have a fieldname. If a fieldname'
           'has less pages than this then it is not reported as a failure even'
           'if the percentage difference is more than the variance threshold.',
      default=50)
  option_parser.add_option(
      '', '--discard_outliers',
      help='Determines the percentage of the outliers that will be discarded'
           'from the top and bottom values. Eg: If this value is 10% and the'
           'number of webpages in a field are 10 then the 1st and 10th'
           'webpages are discarded.',
      default=10)

  options, unused_args = option_parser.parse_args()
  if not (options.csv_file1 and options.csv_file2 and options.output_html_dir
          and options.variance_threshold):
    option_parser.error('Must specify csv_file1, csv_file2, output_html_dir and'
                        ' variance_threshold')

  sys.exit(CsvComparer(
      options.csv_file1, options.csv_file2, options.output_html_dir,
      options.variance_threshold, options.absolute_url,
      options.min_pages_in_each_field, options.discard_outliers).Compare())


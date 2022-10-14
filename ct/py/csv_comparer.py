#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to compare two CSV files and output HTML results."""


import csv
import datetime
import optparse
import os
import re
import sys
import tempfile

# Add the django settings file to DJANGO_SETTINGS_MODULE.
import django
os.environ['DJANGO_SETTINGS_MODULE'] = 'csv-django-settings'
django.setup()
from django.template import loader

GS_HTML_BROWSER_LINK = (
    'https://console.cloud.google.com/storage/browser/cluster-telemetry')
GS_HTML_DIRECT_LINK = (
    'https://console.cloud.google.com/m/cloudstorage/b/cluster-telemetry/o')


def _GetPercentageDiff(value1, value2):
  """Returns the percentage difference between the specified values."""
  difference = value2 - value1
  avg = (value2 + value1)/2
  return 0 if avg == 0 else difference/avg * 100


def _GetPercentageChange(value1, value2):
  """Returns the percentage change between the specified values."""
  difference = value2 - value1
  return 0 if value1 == 0 else difference/value1 * 100


class PageValues(object):
  """Container class to hold the values of a page name."""
  def __init__(self, page_name, value1, value2, perc_diff, perc_change,
               pageset_link, archive_link, traceUrls1, traceUrls2):
    self.page_name = page_name
    self.value1 = value1
    self.value2 = value2
    self.perc_diff = perc_diff
    self.perc_change = perc_change
    self.pageset_link = pageset_link
    self.archive_link = archive_link
    self.traceUrls1 = traceUrls1.split(',') if traceUrls1 else []
    self.traceUrls2 = traceUrls2.split(',') if traceUrls2 else []


class FieldNameValues(object):
  """Container class to hold the values of a field name."""
  def __init__(self, value1, value2, perc_diff, total_webpages_reported):
    self.value1 = value1
    self.value2 = value2
    self.perc_diff = perc_diff
    self.total_webpages_reported = total_webpages_reported


class CsvComparer(object):
  """Class that compares two telemetry CSV files and outputs HTML results."""

  def __init__(self, csv_file1, csv_file2, output_html_dir, requester_email,
               chromium_patch_link, skia_patch_link,
               variance_threshold, absolute_url, min_pages_in_each_field,
               discard_outliers, raw_csv_nopatch, raw_csv_withpatch,
               num_repeated, target_platform, crashed_instances,
               missing_devices, browser_args_nopatch, browser_args_withpatch,
               pageset_type, chromium_hash, skia_hash, missing_output_workers,
               logs_link_prefix, description, total_archives):
    """Constructs a CsvComparer instance."""
    self._csv_file1 = csv_file1
    self._csv_file2 = csv_file2
    self._output_html_dir = output_html_dir
    self._requester_email = requester_email
    self._chromium_patch_link = chromium_patch_link
    self._skia_patch_link = skia_patch_link
    self._variance_threshold = float(variance_threshold)
    self._absolute_url = absolute_url
    self._min_pages_in_each_field = min_pages_in_each_field
    self._discard_outliers = float(discard_outliers)
    self._raw_csv_nopatch = raw_csv_nopatch
    self._raw_csv_withpatch = raw_csv_withpatch
    self._num_repeated = num_repeated
    self._target_platform = target_platform
    self._crashed_instances = crashed_instances
    self._missing_devices = missing_devices
    self._browser_args_nopatch = browser_args_nopatch
    self._browser_args_withpatch = browser_args_withpatch
    self._pageset_type = pageset_type
    self._chromium_hash = chromium_hash
    self._skia_hash = skia_hash
    self._missing_output_workers = missing_output_workers
    self._logs_link_prefix = logs_link_prefix
    self._description = description
    self._total_archives = total_archives

  def _IsPercDiffSameOrAboveThreshold(self, perc_diff):
    """Compares the specified diff to the variance threshold.

    Returns True if the difference is at or above the variance threshold.
    """
    return abs(perc_diff) >= self._variance_threshold

  def _GetSortedCSV(self, unsorted_csv_reader):
    """Sorts the specified CSV by page_name into a new CSV file."""
    result = sorted(unsorted_csv_reader, key=lambda d: d['page_name'])
    fd, sorted_csv_file = tempfile.mkstemp()
    # Close the fd.
    os.close(fd)
    with open(sorted_csv_file, 'w') as f:
      writer = csv.DictWriter(f, unsorted_csv_reader.fieldnames)
      writer.writeheader()
      writer.writerows(result)
    return sorted_csv_file

  def Compare(self):
    """Method that does the CSV comparision."""

    # Do one pass of all the page_names in the 1st CSV and store them.
    # The purpose of this is that when we walk through the 2nd CSV we will know
    # Whether the same page exists in the 1st CSV (the pages are ordered the
    # same way in both files but some could be missing from each file).
    csv1_page_names = {}
    with open(self._csv_file1, 'r') as f1:
      csv1_reader = csv.DictReader(f1)
      for row in csv1_reader:
        csv1_page_names[row['page_name']] = 1

    # Sort both CSVs.
    with open(self._csv_file1, 'r') as f1, open(self._csv_file2, 'r') as f2:
      sorted_csv1_filepath = self._GetSortedCSV(csv.DictReader(f1))
      sorted_csv2_filepath = self._GetSortedCSV(csv.DictReader(f2))
      with open(sorted_csv1_filepath, 'r') as sorted_csv1, \
           open(sorted_csv2_filepath, 'r') as sorted_csv2:
        csv1_reader = csv.DictReader(sorted_csv1)
        csv2_reader = csv.DictReader(sorted_csv2)

        # Dictionary that holds the fieldname to the ongoing total on both CSVs.
        fieldnames_to_totals = {}
        # Map of a fieldname to list of tuples containing (page_name,
        # csv_value1, csv_value2, percentage_difference).
        fieldnames_to_page_values = {}
        # Map of a fieldname to the discarded page value.
        fieldnames_to_discards = {}

        # Now walk through both CSV files with a pointer at each one and collect
        # the value totals.
        for csv2_row in csv2_reader:
          # Make sure the CSV2 page_name existings in CSV1 else skip it (move
          # CSV2 pointer down).
          page_name2 = csv2_row['page_name']
          if page_name2 not in csv1_page_names:
            continue
          # Reach the right page_name in CSV1 (move CSV1 pointer down).
          try:
            csv1_row = next(csv1_reader);
            while csv1_row['page_name'] != page_name2:
              csv1_row = next(csv1_reader)
          except StopIteration:
            # Reached the end of CSV1, break out of the row loop.
            break

          # Store values for all fieldnames (except page_name).
          for fieldname in csv2_reader.fieldnames:
            if fieldname != 'page_name' and fieldname in csv1_row:
              if csv1_row[fieldname] == '' or csv2_row[fieldname] == '':
                # TODO(rmistry): Check with tonyg about what the algorithm
                # should be doing when one CSV has an empty value and the other
                # does not.
                continue
              try:
                if csv1_row[fieldname] == '-':
                  csv1_value = 0
                else:
                  csv1_value = float(csv1_row.get(fieldname))
                if csv2_row[fieldname] == '-':
                  csv2_value = 0
                else:
                  csv2_value = float(csv2_row.get(fieldname))
              except ValueError:
                # We expected only floats, cannot compare strings. Skip field.
                continue

              # Update the total in the dict.
              fieldname_values = fieldnames_to_totals.get(
                  fieldname, FieldNameValues(0, 0, 0, 0))
              fieldname_values.value1 += csv1_value
              fieldname_values.value2 += csv2_value
              fieldnames_to_totals[fieldname] = fieldname_values

              perc_diff = _GetPercentageDiff(csv1_value, csv2_value)
              if self._IsPercDiffSameOrAboveThreshold(perc_diff):
                rank = 1
                worker_num = 1
                m = re.match(r".* \(#([0-9]+)\)", page_name2)
                if m and m.group(1):
                  rank = int(m.group(1))
                  while rank > worker_num * 100:
                    worker_num += 1
                pageset_link = (
                    '%s/swarming/page_sets/%s/%s/%s.py' % (
                        GS_HTML_DIRECT_LINK, self._pageset_type, rank, rank))
                archive_link = (
                    '%s/swarming/webpage_archives/%s/%s' % (
                        GS_HTML_BROWSER_LINK, self._pageset_type, rank))
                # Add this page only if its diff is above the threshold.
                l = fieldnames_to_page_values.get(fieldname, [])
                pv = PageValues(
                    page_name2, csv1_value, csv2_value, perc_diff,
                    _GetPercentageChange(csv1_value, csv2_value), pageset_link,
                    archive_link, csv1_row.get('traceUrls'),
                    csv2_row.get('traceUrls'))
                l.append(pv)
                fieldnames_to_page_values[fieldname] = l
      os.remove(sorted_csv1_filepath)
      os.remove(sorted_csv2_filepath)

    # Calculate and add the percentage differences for each fieldname.
    # The fieldnames_to_totals dict is modified in the below loop to remove
    # entries which are below the threshold .
    for fieldname in list(fieldnames_to_totals):
      fieldname_values = fieldnames_to_totals[fieldname]
      if fieldname not in fieldnames_to_page_values:
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
        fieldnames_to_discards[fieldname] = top_outliers + bottom_outliers

      perc_diff = _GetPercentageDiff(fieldname_values.value1,
                                     fieldname_values.value2)
      if self._IsPercDiffSameOrAboveThreshold(perc_diff):
        if (len(fieldnames_to_page_values[fieldname]) <
            self._min_pages_in_each_field):
          # This field does not have enough webpages, delete it from both maps.
          print('Removing because not enough webpages: %s' % fieldname)
          print(len(fieldnames_to_page_values[fieldname]))
          del fieldnames_to_page_values[fieldname]
          del fieldnames_to_totals[fieldname]
          continue
        fieldname_values.perc_diff = perc_diff
        fieldname_values.perc_change = _GetPercentageChange(
            fieldname_values.value1, fieldname_values.value2)
      else:
        # Only store fieldnames that are below the variance threshold.
        print('Removing because below the variance threshold: %s' % fieldname)
        del fieldnames_to_totals[fieldname]

    # Delete keys in fieldnames_to_page_values that are not in
    # fieldnames_to_totals because those are the only ones we want to
    # display.
    fieldnames_to_page_values = dict(
        (k,v) for k,v in fieldnames_to_page_values.items()
        if k in fieldnames_to_totals)

    # Both maps should end up with the same number of keys.
    assert set(fieldnames_to_page_values.keys()) == set(
        fieldnames_to_totals.keys())

    # Set the number of reporting webpages in fieldnames_to_totals.
    for fieldname, values in fieldnames_to_page_values.items():
      fieldnames_to_totals[fieldname].total_webpages_reported = len(values)

    # Done processing. Output the HTML.
    self.OutputToHTML(fieldnames_to_totals, fieldnames_to_page_values,
                      fieldnames_to_discards, self._output_html_dir)

  def OutputToHTML(self, fieldnames_to_totals, fieldnames_to_page_values,
                   fieldnames_to_discards, html_dir):
    # Calculate the current UTC time.
    html_report_date = datetime.datetime.utcnow().strftime('%Y-%m-%d %H:%M UTC')
    # Output the main totals HTML page.
    sorted_fieldnames_totals_items = sorted(
        fieldnames_to_totals.items(), key=lambda tuple: tuple[1].perc_diff,
        reverse=True)
    missing_output_workers_list = []
    if self._missing_output_workers:
      missing_output_workers_list = self._missing_output_workers.split(' ')
    rendered = loader.render_to_string(
        'csv_totals.html',
        {'sorted_fieldnames_totals_items': sorted_fieldnames_totals_items,
         'requester_email': self._requester_email,
         'chromium_patch_link': self._chromium_patch_link,
         'skia_patch_link': self._skia_patch_link,
         'raw_csv_nopatch': self._raw_csv_nopatch,
         'raw_csv_withpatch': self._raw_csv_withpatch,
         'threshold': self._variance_threshold,
         'discard_outliers': self._discard_outliers,
         'min_webpages': self._min_pages_in_each_field,
         'num_repeated': self._num_repeated,
         'target_platform': self._target_platform,
         'crashed_instances': self._crashed_instances,
         'missing_devices': self._missing_devices,
         'browser_args_nopatch': self._browser_args_nopatch,
         'browser_args_withpatch': self._browser_args_withpatch,
         'absolute_url': self._absolute_url,
         'pageset_type': self._pageset_type,
         'html_report_date': html_report_date,
         'chromium_hash': self._chromium_hash,
         'skia_hash': self._skia_hash,
         'missing_output_workers': missing_output_workers_list,
         'logs_link_prefix': self._logs_link_prefix,
         'description': self._description,
        })
    index_html_path = os.path.join(self._output_html_dir, 'index.html')
    with open(index_html_path, 'w') as index_html:
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
           'discard_outliers': self._discard_outliers,
           'discarded_webpages': fieldnames_to_discards.get(fieldname, []),
           'total_archives': self._total_archives,
           'absolute_url': self._absolute_url})
      with open(
          os.path.join(self._output_html_dir,
          'fieldname%s.html' % fieldname_count), 'w') as fieldname_html:
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
      '', '--requester_email',
      help='Email address of the user who kicked off the run.')
  option_parser.add_option(
      '', '--chromium_patch_link',
      help='Link to the Chromium patch used for this run.')
  option_parser.add_option(
      '', '--skia_patch_link',
      help='Link to the Skia patch used for this run.')
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
      default=0)
  option_parser.add_option(
      '', '--discard_outliers',
      help='Determines the percentage of the outliers that will be discarded'
           'from the top and bottom values. Eg: If this value is 10% and the'
           'number of webpages in a field are 10 then the 1st and 10th'
           'webpages are discarded.',
      default=10)
  option_parser.add_option(
      '', '--num_repeated',
      help='The number of times each pageset was run.')
  option_parser.add_option(
      '', '--raw_csv_nopatch',
      help='Link to the raw CSV output of the nopatch run.')
  option_parser.add_option(
      '', '--raw_csv_withpatch',
      help='Link to the raw CSV output of the withpatch run.')
  option_parser.add_option(
      '', '--crashed_instances',
      help='Text that lists any crashed instances.')
  option_parser.add_option(
      '', '--missing_devices',
      help='Text that lists all instances with missing Android devices.')
  option_parser.add_option(
      '', '--target_platform',
      help='The platform telemetry benchmarks/measurements were run on.')
  option_parser.add_option(
      '', '--browser_args_nopatch',
      help='The browser args that were used for the nopatch run.')
  option_parser.add_option(
      '', '--browser_args_withpatch',
      help='The browser args that were used for the withpatch run.')
  option_parser.add_option(
      '', '--pageset_type',
      help='The page set type this run was done on.')
  option_parser.add_option(
      '', '--chromium_hash',
      help='The chromium git hash that was used for this run.')
  option_parser.add_option(
      '', '--skia_hash',
      help='The skia git hash that was used for this run.',
      default='')
  option_parser.add_option(
      '', '--missing_output_workers',
      help='Workers which had no output for this run.')
  option_parser.add_option(
      '', '--logs_link_prefix',
      help='Prefix link to the logs of the workers.')
  option_parser.add_option(
      '', '--description',
      help='The description of the run as entered by the requester.')
  option_parser.add_option(
      '', '--total_archives',
      help='Number of archives that were used to get these results.')

  options, unused_args = option_parser.parse_args()
  if not (options.csv_file1 and options.csv_file2 and options.output_html_dir
          and options.variance_threshold and options.requester_email
          and options.chromium_patch_link
          and options.skia_patch_link and options.raw_csv_nopatch
          and options.raw_csv_withpatch and options.num_repeated
          and options.target_platform and options.pageset_type
          and options.chromium_hash and options.description):
    option_parser.error('Must specify csv_file1, csv_file2, output_html_dir, '
                        'variance_threshold, requester_email, '
                        'chromium_patch_link, '
                        'skia_patch_link, raw_csv_nopatch, description, '
                        'raw_csv_withpatch, num_repeated, pageset_type, '
                        'chromium_hash and target_platform')

  sys.exit(CsvComparer(
      options.csv_file1, options.csv_file2, options.output_html_dir,
      options.requester_email, options.chromium_patch_link,
      options.skia_patch_link,
      options.variance_threshold, options.absolute_url,
      options.min_pages_in_each_field, options.discard_outliers,
      options.raw_csv_nopatch, options.raw_csv_withpatch,
      options.num_repeated, options.target_platform,
      options.crashed_instances, options.missing_devices,
      options.browser_args_nopatch, options.browser_args_withpatch,
      options.pageset_type, options.chromium_hash, options.skia_hash,
      options.missing_output_workers, options.logs_link_prefix,
      options.description, options.total_archives).Compare())


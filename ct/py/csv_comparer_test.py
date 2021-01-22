#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module csv_merger."""

import datetime
import filecmp
import os
import shutil
import tempfile
import unittest

import csv_comparer


class TestCsvComparer(unittest.TestCase):

  def setUp(self):
    self._test_csv_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)),
        'test_data', 'csv_comparer')
    self._actual_output_dir = tempfile.mkdtemp()

    # Set mocks.
    class _MockUtcNow(object):
      @staticmethod
      def strftime(format_str):
        self.assertEqual('%Y-%m-%d %H:%M UTC', format_str)
        return '2014-05-19 16:50 UTC'
    class _MockDatetime(object):
      @staticmethod
      def utcnow():
        return _MockUtcNow()
    self._original_datetime = datetime.datetime
    datetime.datetime = _MockDatetime

  def tearDown(self):
    shutil.rmtree(self._actual_output_dir)
    datetime.datetime = self._original_datetime

  def _AssertHTMLFiles(self, sub_dir, additional_files=()):
    # Ensure that the two html files we care about are as expected.
    for html_file in ('index.html', 'fieldname1.html') + additional_files:
      self.assertTrue(
          filecmp.cmp(os.path.join(self._test_csv_dir, sub_dir, html_file),
                      os.path.join(self._actual_output_dir, html_file)))

  def test_E2EComparerWithDiscardOutliers(self):
    comparer = csv_comparer.CsvComparer(
        csv_file1=os.path.join(self._test_csv_dir, 'comparer_csv1.csv'),
        csv_file2=os.path.join(self._test_csv_dir, 'comparer_csv2.csv'),
        output_html_dir=self._actual_output_dir,
        requester_email='superman@krypton.com',
        chromium_patch_link='http://chromium-patch.com',
        skia_patch_link='http://skia-patch.com',
        raw_csv_nopatch='http://raw-csv-nopatch.com',
        raw_csv_withpatch='http://raw-csv-withpatch.com',
        variance_threshold=10,
        absolute_url='',
        min_pages_in_each_field=1,
        discard_outliers=12.5,
        num_repeated=3,
        target_platform='Android',
        crashed_instances='build1-b5 build10-b5',
        missing_devices='build99-b5 build100-b5',
        browser_args_nopatch='--test=1',
        browser_args_withpatch='--test=2',
        pageset_type='Mobile10k',
        chromium_hash='abcdefg1234567',
        skia_hash='tuvwxyz1234567',
        missing_output_workers='1 3 100',
        logs_link_prefix=('https://chrome-swarming.appspot.com/tasklist?'
                          'l=500&f=runid:testing&f=name:perf_task_'),
        description='E2EComparerWithDiscardOutliers',
        total_archives='',
    )
    comparer.Compare()
    self._AssertHTMLFiles('discard_outliers')

  def test_E2EComparerWithNoDiscardOutliers(self):
    comparer = csv_comparer.CsvComparer(
        csv_file1=os.path.join(self._test_csv_dir, 'comparer_csv1.csv'),
        csv_file2=os.path.join(self._test_csv_dir, 'comparer_csv2.csv'),
        output_html_dir=self._actual_output_dir,
        requester_email='superman@krypton.com',
        chromium_patch_link='http://chromium-patch.com',
        skia_patch_link='http://skia-patch.com',
        raw_csv_nopatch='http://raw-csv-nopatch.com',
        raw_csv_withpatch='http://raw-csv-withpatch.com',
        variance_threshold=0,
        absolute_url='',
        min_pages_in_each_field=0,
        discard_outliers=0,
        num_repeated=3,
        target_platform='Linux',
        crashed_instances='',
        missing_devices='',
        browser_args_nopatch='',
        browser_args_withpatch='',
        pageset_type='10k',
        chromium_hash='abcdefg1234567',
        skia_hash='tuvwxyz1234567',
        missing_output_workers='',
        logs_link_prefix='',
        description='E2EComparerWithNoDiscardOutliers',
        total_archives='10',
    )
    comparer.Compare()
    self._AssertHTMLFiles('keep_outliers',
                          ('fieldname2.html', 'fieldname3.html'))


if __name__ == '__main__':
  unittest.main()


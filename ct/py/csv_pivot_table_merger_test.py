#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module csv_pivot_table_merger."""

import csv_pivot_table_merger
import os
import unittest


ACTUAL_OUTPUT_FILENAME = 'actual_output'


class TestCsvMerger(unittest.TestCase):

  def setUp(self):
    self._test_csv_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)),
        'test_data', 'csv_pivot_table_merger')
    self._actual_output = os.path.join(self._test_csv_dir,
                                       ACTUAL_OUTPUT_FILENAME)

  def tearDown(self):
    os.remove(self._actual_output)

  def test_E2EMerger(self):
    merger = csv_pivot_table_merger.CsvMerger(
        csv_dir=self._test_csv_dir, output_csv_name=ACTUAL_OUTPUT_FILENAME,
        value_column_name='avg', handle_strings=False)
    merger.Merge()

    # Compare actual with expected.
    expected_output = os.path.join(self._test_csv_dir, 'expected_output')
    with open(expected_output, 'rb') as f:
      expected_output_lines = f.readlines()
    with open(self._actual_output, 'rb') as f:
        actual_output_lines = f.readlines()
    self.assertTrue(set(expected_output_lines) == set(actual_output_lines))

  def test_E2EMergerWithDiffColName(self):
    merger = csv_pivot_table_merger.CsvMerger(
        csv_dir=self._test_csv_dir, output_csv_name=ACTUAL_OUTPUT_FILENAME,
        value_column_name='pct_001', handle_strings=False)
    merger.Merge()

    # Compare actual with expected.
    expected_output = os.path.join(self._test_csv_dir,
                                   'expected_output_diff_col_name')
    with open(expected_output, 'rb') as f:
      expected_output_lines = f.readlines()
    with open(self._actual_output, 'rb') as f:
      actual_output_lines = f.readlines()
    self.assertTrue(set(expected_output_lines) == set(actual_output_lines))

  def test_E2EMergerWithStrings(self):
    merger = csv_pivot_table_merger.CsvMerger(
        csv_dir=self._test_csv_dir, output_csv_name=ACTUAL_OUTPUT_FILENAME,
        value_column_name='avg', handle_strings=True)
    merger.Merge()

    # Compare actual with expected.
    expected_output = os.path.join(self._test_csv_dir,
                                   'expected_output_with_strings')
    with open(expected_output, 'rb') as f:
      expected_output_lines = f.readlines()
    with open(self._actual_output, 'rb') as f:
      actual_output_lines = f.readlines()
    self.assertTrue(set(expected_output_lines) == set(actual_output_lines))


if __name__ == '__main__':
  unittest.main()


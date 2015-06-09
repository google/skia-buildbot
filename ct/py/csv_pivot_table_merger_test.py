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
        csv_dir=self._test_csv_dir, output_csv_name=ACTUAL_OUTPUT_FILENAME)
    merger.Merge()

    # Compare actual with expected.
    expected_output = os.path.join(self._test_csv_dir, 'expected_output')
    expected_output_lines = open(expected_output).readlines()
    actual_output_lines = open(self._actual_output).readlines()
    self.assertTrue(set(expected_output_lines) == set(actual_output_lines))


if __name__ == '__main__':
  unittest.main()


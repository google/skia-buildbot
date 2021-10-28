#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module csv_merger."""

import csv
import csv_merger
import os
import tempfile
import test_utils
import unittest


ACTUAL_OUTPUT_FILENAME = 'actual_output'


class TestCsvMerger(unittest.TestCase):

  def setUp(self):
    self._test_csv_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)),
        'test_data', 'csv_merger')
    self._actual_output = os.path.join(tempfile.mkdtemp(),
                                       ACTUAL_OUTPUT_FILENAME)

  def tearDown(self):
    os.remove(self._actual_output)

  def test_E2EMerger(self):
    merger = csv_merger.CsvMerger(csv_dir=self._test_csv_dir,
                                  output_csv_name=self._actual_output,
                                  handle_strings=False)
    merger.Merge()

    # Compare actual with expected.
    expected_csv = os.path.join(self._test_csv_dir, 'expected_output')
    test_utils.assertCSVs(expected_csv, self._actual_output)

  def test_E2EMergerWithStrings(self):
    merger = csv_merger.CsvMerger(csv_dir=self._test_csv_dir,
                                  output_csv_name=self._actual_output,
                                  handle_strings=True)
    merger.Merge()

    # Compare actual with expected.
    expected_csv = os.path.join(self._test_csv_dir,
                                   'expected_output_with_strings')
    test_utils.assertCSVs(expected_csv, self._actual_output)


if __name__ == '__main__':
  unittest.main()


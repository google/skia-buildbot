#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module csv_merger."""

import csv
import csv_merger
import os
import unittest


ACTUAL_OUTPUT_FILENAME = 'actual_output'


def assertCSVs(expected_csv, actual_csv):
  with open(expected_csv, 'r') as f:
    expected_output_list = list(csv.DictReader(f))
  with open(actual_csv, 'r') as f:
    actual_output_list = list(csv.DictReader(f))
  assert len(expected_output_list) == len(actual_output_list)
  alreadyMatchedIndices = []
  for i in range(len(expected_output_list)):
    for j in range(len(actual_output_list)):
      if j in alreadyMatchedIndices:
        continue
      if(expected_output_list[i] == actual_output_list[j]):
        alreadyMatchedIndices.append(j)
        break
    else:
      raise AssertionError("%s and %s are not equal" % (
                               expected_csv, actual_csv))


class TestCsvMerger(unittest.TestCase):

  def setUp(self):
    self._test_csv_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)),
        'test_data', 'csv_merger')
    self._actual_output = os.path.join(self._test_csv_dir,
                                       ACTUAL_OUTPUT_FILENAME)

  def tearDown(self):
    os.remove(self._actual_output)

  def test_E2EMerger(self):
    merger = csv_merger.CsvMerger(csv_dir=self._test_csv_dir,
                                  output_csv_name=ACTUAL_OUTPUT_FILENAME,
                                  handle_strings=False)
    merger.Merge()

    # Compare actual with expected.
    expected_csv = os.path.join(self._test_csv_dir, 'expected_output')
    assertCSVs(expected_csv, self._actual_output)

  def test_E2EMergerWithStrings(self):
    merger = csv_merger.CsvMerger(csv_dir=self._test_csv_dir,
                                  output_csv_name=ACTUAL_OUTPUT_FILENAME,
                                  handle_strings=True)
    merger.Merge()

    # Compare actual with expected.
    expected_csv = os.path.join(self._test_csv_dir,
                                   'expected_output_with_strings')
    assertCSVs(expected_csv, self._actual_output)


if __name__ == '__main__':
  unittest.main()


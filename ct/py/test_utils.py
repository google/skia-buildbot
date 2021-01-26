#!/usr/bin/env python
# Copyright (c) 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utilities for CT python tests.."""

import csv


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

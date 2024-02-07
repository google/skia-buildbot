#!/usr/bin/env python
# Copyright (c) 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utilities for CT python tests.."""


def assertFiles(expected_file, actual_file):
  with open(expected_file, 'r') as f:
    expected_output = f.read()
  with open(actual_file, 'r') as f:
    actual_output = f.read()
  assert expected_output == actual_output, (
      f"Expected:\n{expected_output}\nActual:\n{actual_output}")

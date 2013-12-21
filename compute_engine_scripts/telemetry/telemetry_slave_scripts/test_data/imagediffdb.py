#!/usr/bin/env python                                                           
# Copyright (c) 2013 The Chromium Authors. All rights reserved.                 
# Use of this source code is governed by a BSD-style license that can be        
# found in the LICENSE file.

"""Dummy module that pretends to be imagediffdb.py for write_json_summary_test.

imagediffdb.py here refers to
https://code.google.com/p/skia/source/browse/trunk/gm/rebaseline_server/imagediffdb.py
"""


class DiffRecord(object):

  def __init__(self, storage_root, expected_image_url, expected_image_locator,
               actual_image_url, actual_image_locator, expected_images_subdir,
               actual_images_subdir):
    pass

  def get_num_pixels_differing(self):
    return 1

  def get_percent_pixels_differing(self):
    return 2

  def get_weighted_diff_measure(self):
    return 3

  def get_max_diff_per_channel(self):
    return 4


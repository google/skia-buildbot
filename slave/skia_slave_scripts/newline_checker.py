#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Newline checking step """

import os
import sys


_FILE_SUFFIXES_TO_CHECK = ['.cpp', '.h', '.c']

_SUBDIRS_TO_IGNORE = ['.svn', 'third_party']


def NewlineChecker():
  files_without_trailing_newlines = _ListFiles(
      os.getcwd())  # Assuming current directory is trunk.
  if files_without_trailing_newlines:
    raise Exception(
        'The following file(s) have no newlines at the end:\n %s' % (
            '\n'.join(files_without_trailing_newlines)));


def _ListFiles(directory):
  files_without_trailing_newlines = []
  for item in os.listdir(directory):
    full_item_path = os.path.join(directory, item)
    if os.path.isfile(full_item_path):
      for suffix in _FILE_SUFFIXES_TO_CHECK:
        if item.endswith(suffix):
          file_content = file(full_item_path).read();
          if file_content and file_content[-1] != '\n':
            files_without_trailing_newlines.append(full_item_path)
    elif item not in _SUBDIRS_TO_IGNORE:
      files_without_trailing_newlines.extend(_ListFiles(full_item_path))
  return files_without_trailing_newlines


if '__main__' == __name__:
  sys.exit(NewlineChecker())


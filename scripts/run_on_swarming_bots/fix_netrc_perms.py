#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Fix .netrc permissions if necessary."""


import os


home = os.path.expanduser('~')
expected_locations = [home]
netrc_file = '.netrc'
if os.name == 'nt':
  netrc_file = '_netrc'

os.chmod(os.path.join(home, netrc_file), 0600)

#!/usr/bin/env python                                                           
# Copyright (c) 2013 The Chromium Authors. All rights reserved.                 
# Use of this source code is governed by a BSD-style license that can be        
# found in the LICENSE file.

"""Dummy module that pretends to be gm_json.py for write_json_summary_test.

gm_json.py here refers to
https://code.google.com/p/skia/source/browse/trunk/gm/gm_json.py
"""

import json


JSONKEY_ACTUALRESULTS = 'actual-results'

JSONKEY_ACTUALRESULTS_NOCOMPARISON = 'no-comparison'

def LoadFromFile(file_path):
  return json.loads(open(file_path, 'r').read())

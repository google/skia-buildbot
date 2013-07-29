#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This file just imports slaves.cfg so that the information contained can be
easily accessed. """

import imp
_slaves_cfg_file = imp.load_source('slaves_cfg_file', 'slaves.cfg')

SLAVES = _slaves_cfg_file.slaves

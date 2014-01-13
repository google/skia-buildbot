#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This file just imports slaves.cfg so that the information contained can be
easily accessed. """

import imp
import os


_path_to_slaves_cfg = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                                   'slaves.cfg'))
_slaves_cfg_file = imp.load_source('slaves_cfg_file', _path_to_slaves_cfg)

SLAVES = _slaves_cfg_file.slaves

CQ_TRYBOTS = _slaves_cfg_file.cq_trybots

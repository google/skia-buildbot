#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on all buildslaves on this machine."""


import sys

import run_cmd


if '__main__' == __name__:
  print run_cmd.encode_results(run_cmd.run_on_local_slaves(sys.argv[1:]))

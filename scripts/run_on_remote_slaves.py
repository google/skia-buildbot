#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on all buildslaves on this machine."""


import run_cmd


if '__main__' == __name__:
  options = run_cmd.parse_args(positional_args=['host'])
  results = run_cmd.run_on_remote_slaves(options.host, options.cmd)
  results.print_results(pretty=options.pretty)

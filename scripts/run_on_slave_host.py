#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on a build slave host machine, listed in slave_hosts_cfg."""


import run_cmd


if '__main__' == __name__:
  options = run_cmd.parse_args(positional_args=['host'])
  results = run_cmd.run_on_remote_host(options.host, options.cmd)
  run_cmd.print_results(results, options.pretty)

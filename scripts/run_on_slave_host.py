#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command on a build slave host machine, listed in slave_hosts_cfg."""


import sys

import run_cmd


if '__main__' == __name__:
  if len(sys.argv) < 3:
    sys.stderr.write('Usage: %s <slave_host_name> <command>\n' % __file__)
    sys.exit(1)
  print run_cmd.encode_results(run_cmd.run_on_remote_host(sys.argv[1],
                                                          sys.argv[2:]))

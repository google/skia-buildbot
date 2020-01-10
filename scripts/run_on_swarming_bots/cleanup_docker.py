#!/usr/bin/env python
#
# Copyright 2020 Google LLC.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Clean up all docker caches."""

import subprocess

print(subprocess.check_output(['docker', 'system', 'prune', '-fa']))
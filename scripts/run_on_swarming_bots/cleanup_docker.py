#!/usr/bin/env python
#
# Copyright 2020 Google LLC.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Clean up all docker caches."""

import subprocess

# Be sure to include -a, otherwise volumes and the nebulous overlay2
# will not be cleaned up. overlay2 was noticed to be a very large
# component, over 150GB large in some cases.
print(subprocess.check_output(['docker', 'system', 'prune', '-fa']))
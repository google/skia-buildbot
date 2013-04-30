# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verifies that the configuration for each BuildFactory matches its
expectation. """


import os
import sys

sys.path.append('master')
sys.path.append('site_config')
sys.path.append(os.path.join('third_party', 'chromium_buildbot', 'scripts'))
sys.path.append(os.path.join('third_party', 'chromium_buildbot', 'site_config'))
sys.path.append(os.path.join('third_party', 'chromium_buildbot', 'third_party',
                             'buildbot_8_4p1'))

import config
import master_builders_cfg


def main():
  c = {}
  c['schedulers'] = []
  c['builders'] = []
  master_builders_cfg.Update(config, config.Master.Skia, c)


if '__main__' == __name__:
  sys.exit(main())
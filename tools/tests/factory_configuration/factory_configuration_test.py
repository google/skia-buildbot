# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verifies that the configuration for each BuildFactory matches its
expectation. """


import os
import sys

buildbot_path = os.path.join(os.path.abspath(os.path.dirname(__file__)),
                             os.pardir, os.pardir, os.pardir)
sys.path.append(os.path.join(buildbot_path, 'master'))
sys.path.append(os.path.join(buildbot_path, 'site_config'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'scripts'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'site_config'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'buildbot_8_4p1'))

import config
import config_private
import master_builders_cfg


def main():
  c = {}
  c['schedulers'] = []
  c['builders'] = []

  # Make sure that the configuration errors out if validation fails.
  config_private.die_on_validation_failure = True

  # Pretend that the master is the production master, so that the tested
  # configuration is identical to that of the production master.
  config.Master.Skia.is_production_host = True

  # Run the configuration.
  master_builders_cfg.Update(config, config.Master.Skia, c)


if '__main__' == __name__:
  sys.exit(main())
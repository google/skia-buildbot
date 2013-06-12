# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verifies that the configuration for each BuildFactory matches its
expectation. """


from distutils import dir_util
import os
import sys

my_path = os.path.abspath(os.path.dirname(__file__))
buildbot_path = os.path.join(my_path, os.pardir, os.pardir, os.pardir)
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


def RunTest(die_on_validation_failure=True):
  c = {}
  c['schedulers'] = []
  c['builders'] = []

  # Make sure that the configuration errors out if validation fails.
  config_private.die_on_validation_failure = die_on_validation_failure

  # Pretend that the master is the production master, so that the tested
  # configuration is identical to that of the production master.
  config.Master.Skia.is_production_host = True

  # Run the configuration.
  master_builders_cfg.Update(config, config.Master.Skia, c)


def main():
  if '--rebaseline' in sys.argv:
    print 'Generating new actuals.'
    RunTest(die_on_validation_failure=False)
    print 'Copying actual to expected.'
    dir_util.copy_tree(os.path.join(my_path, 'actual'),
                       os.path.join(my_path, 'expected'))
  else:
    print 'Validating factory configuration.'
    RunTest()

if '__main__' == __name__:
  sys.exit(main())
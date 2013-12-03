# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verifies that the configuration for each BuildFactory matches its
expectation. """


from distutils import dir_util
import imp
import os
import shutil
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
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'jinja2'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'twisted_8_1'))


import config
import config_private


def RunTest(die_on_validation_failure=True):
  c = {}
  c['schedulers'] = []
  c['builders'] = []

  # Make sure that the configuration errors out if validation fails.
  config_private.die_on_validation_failure = die_on_validation_failure

  # Pretend that the master is the production master, so that the tested
  # configuration is identical to that of the production master.
  config.Master.Skia.is_production_host = True

  # Move to the .../buildbot/master directory, which is what the build master
  # expects the CWD to be.
  os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), os.pardir,
                        os.pardir, os.pardir, 'master'))

  # Run the configuration. The setup in master.cfg runs when the module is
  # imported, so this import is roughly equivalent to a function call. We have
  # to use the imp module because master.cfg is not a .py file.
  imp.load_source('master_cfg', 'master.cfg')


def main():
  if '--rebaseline' in sys.argv:
    print 'Generating new actuals.'
    if os.path.exists(os.path.join(my_path, 'actual')):
      shutil.rmtree(os.path.join(my_path, 'actual'))
    os.makedirs(os.path.join(my_path, 'actual'))
    RunTest(die_on_validation_failure=False)
    print 'Copying actual to expected.'
    dir_util.copy_tree(os.path.join(my_path, 'actual'),
                       os.path.join(my_path, 'expected'))
  else:
    print 'Validating factory configuration.'
    RunTest()

if '__main__' == __name__:
  sys.exit(main())
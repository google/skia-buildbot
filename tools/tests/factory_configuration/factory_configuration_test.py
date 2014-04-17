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
  # Create a dummy password file if necessary.
  for password_file in ('.skia_buildbots_password', '.code_review_password',
                        '.status_password'):
    password_path = os.path.join(buildbot_path, 'master', password_file)
    if not os.path.isfile(password_path):
      with open(password_path, 'w') as f:
        f.write('dummy_password')

  # Run the factory config test for each master.
  for build_master_class in config.Master.valid_masters:
    build_master_name = build_master_class.__name__
    print build_master_name
    os.environ['TESTING_MASTER'] = build_master_name

    c = {}
    c['schedulers'] = []
    c['builders'] = []

    # Make sure that the configuration errors out if validation fails.
    config_private.die_on_validation_failure = die_on_validation_failure

    # Pretend that the master is the production master, so that the tested
    # configuration is identical to that of the production master.
    build_master_class.is_production_host = True

    # Move to the .../buildbot/master directory, which is what the build master
    # expects the CWD to be.
    os.chdir(os.path.join(buildbot_path, 'master'))

    # Run the configuration. The setup in master.cfg runs when the module is
    # imported, so this import is roughly equivalent to a function call. We have
    # to use the imp module because master.cfg is not a .py file.
    imp.load_source('master_cfg', 'master.cfg')


def main():
  # While running our test, ignore this environment variable.
  # It will remain set in the user's environment, once this program exits.
  os.environ[config_private.SKIPSTEPS_ENVIRONMENT_VARIABLE] = ''

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
    print 'Validating factory configuration:'
    RunTest()

if '__main__' == __name__:
  sys.exit(main())

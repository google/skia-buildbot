#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" This test verifies that the slave_hosts.cfg and slaves.cfg files are sane
and compatible. """


import os
import sys
import unittest

buildbot_path = os.path.join(os.path.abspath(os.path.dirname(__file__)),
                             os.pardir)
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'site_config'))
from config_private import SKIA_SVN_BASEURL


SLAVES_CFG = os.path.join(buildbot_path, 'master', 'slaves.cfg')
SLAVE_HOSTS_CFG = os.path.join(buildbot_path, 'site_config', 'slave_hosts.cfg')
BUILDBOT_SVN_URL = SKIA_SVN_BASEURL + '/buildbot'


class SlaveHostsCfgTest(unittest.TestCase):
  """ This test verifies that the slave_hosts.cfg and slaves.cfg files are
  sane and compatible. """

  def runTest(self):
    """ Run the test. """

    # First, read both files.
    slaves_cfg = {}
    execfile(SLAVES_CFG, slaves_cfg)

    slave_hosts_cfg = {}
    execfile(SLAVE_HOSTS_CFG, slave_hosts_cfg)

    slaves = slaves_cfg['slaves']
    slave_hosts = slave_hosts_cfg['SLAVE_HOSTS']

    # Verify that every slave listed by a slave host is defined in slaves.cfg.
    for slave_host_data in slave_hosts.itervalues():
      for slave in slave_host_data['slaves']:
        found_slave = False
        for slave_cfg in slaves:
          if slave_cfg['hostname'] == slave:
            found_slave = True
            break
        self.assertTrue(found_slave, 'Unknown slavename: %s' % slave)


if __name__ == '__main__':
  unittest.main()

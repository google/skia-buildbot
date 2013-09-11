#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" This test verifies that the slave_hosts_cfg.py and slaves.cfg files are sane
and compatible. """


import os
import sys
import unittest

import slave_hosts_cfg

buildbot_path = os.path.join(os.path.abspath(os.path.dirname(__file__)),
                             os.pardir)
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'site_config'))

SLAVES_CFG = os.path.join(buildbot_path, 'master', 'slaves.cfg')


class SlaveHostsCfgTest(unittest.TestCase):
  """ This test verifies that the slave_hosts.cfg and slaves.cfg files are
  sane and compatible. """

  def runTest(self):
    """ Run the test. """

    # First, read the slaves.cfg file.
    slaves_cfg = {}
    execfile(SLAVES_CFG, slaves_cfg)
    slaves = slaves_cfg['slaves']

    # Verify that every slave listed by a slave host is defined exactly once in
    # slaves.cfg.
    for slave_host_data in slave_hosts_cfg.SLAVE_HOSTS.itervalues():
      for slave_name, _ in slave_host_data['slaves']:
        found_slave = False
        for slave_cfg in slaves:
          if slave_cfg['hostname'] == slave_name:
            self.assertFalse(found_slave,
                'Slave %s is defined more than once in slaves.cfg' % slave_name)
            found_slave = True
        self.assertTrue(found_slave,
                        'Slave %s is not defined in slaves.cfg' % slave_name)

    # Verify that every slave listed in slaves.cfg is associated with exactly
    # one host in slave_hosts_cfg.py.
    for slave_cfg in slaves:
      found_slave = False
      for slave_host_data in slave_hosts_cfg.SLAVE_HOSTS.itervalues():
        for slave_name, _ in slave_host_data['slaves']:
          if slave_cfg['hostname'] == slave_name:
            self.assertFalse(found_slave,
                'Slave %s is listed for more than one host' % slave_name)
            found_slave = True
      self.assertTrue(found_slave,
                      ('Slave %s is not defined in slaves_hosts_cfg.py' %
                          slave_cfg['hostname']))

    # Verify that the ID for each slave on a given host is unique.
    for slave_host_data in slave_hosts_cfg.SLAVE_HOSTS.itervalues():
      known_ids = []
      for slave_name, slave_id in slave_host_data['slaves']:
        self.assertFalse(slave_id in known_ids,
            'Slave %s has non-unique id %s' % (slave_name, slave_id))

if __name__ == '__main__':
  unittest.main()

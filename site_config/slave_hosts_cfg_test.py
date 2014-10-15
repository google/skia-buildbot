#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" This test verifies that the slave_hosts_cfg.py and slaves.cfg files are sane
and compatible. """


import base64
import unittest
import urllib2

import slave_hosts_cfg


SKIA_PUBLIC_MASTERS = [
  'master.client.skia',
  'master.client.skia.android',
  'master.client.skia.compile',
  'master.client.skia.fyi',
]

SKIA_PRIVATE_MASTERS = [
  # Disable for now, since we don't know how to urlopen the internal code.
  # 'master.client.skia.internal',
]

SLAVES_CFG_PUBLIC_URL = ('https://chromium.googlesource.com/chromium/tools/'
                         'build/+/master/masters/%s/slaves.cfg')
SLAVES_CFG_PRIVATE_URL = ('https://chrome-internal.googlesource.com/chrome/'
                          'tools/build/+/master/masters/%s/slaves.cfg')

SLAVES_CFG_URLS = ([SLAVES_CFG_PUBLIC_URL % m for m in SKIA_PUBLIC_MASTERS] +
                   [SLAVES_CFG_PRIVATE_URL % m for m in SKIA_PRIVATE_MASTERS])


# Since we can't access the internal slaves.cfg, we have to allow some slaves
# to fail without failing the test.
ALLOW_FAILURE_SLAVES = [
  'skia-android-canary',
  'skiabot-shuttle-ubuntu12-arm64-001',
]


def read_slaves_cfg(slaves_cfg_url):
  url = slaves_cfg_url + '?format=TEXT'
  contents = base64.b64decode(urllib2.urlopen(url).read())
  slaves_cfg = {}
  exec(contents, slaves_cfg)
  return slaves_cfg['slaves']


class SlaveHostsCfgTest(unittest.TestCase):
  """ This test verifies that the slave_hosts.cfg and slaves.cfg files are
  sane and compatible. """

  def runTest(self):
    """ Run the test. """

    # First, read the slaves.cfg files.
    slaves = []
    for slaves_cfg_url in SLAVES_CFG_URLS:
      slaves.extend(read_slaves_cfg(slaves_cfg_url))

    # Verify that every slave listed by a slave host is defined exactly once in
    # slaves.cfg.
    for slave_host_data in slave_hosts_cfg.SLAVE_HOSTS.itervalues():
      for slave_name, _, _ in slave_host_data.slaves:
        found_slave = False
        for slave_cfg in slaves:
          if slave_cfg['hostname'] == slave_name:
            self.assertFalse(found_slave,
                'Slave %s is defined more than once in slaves.cfg' % slave_name)
            found_slave = True
        if not found_slave and slave_name in ALLOW_FAILURE_SLAVES:
          continue
        self.assertTrue(found_slave,
                        'Slave %s is not defined in slaves.cfg' % slave_name)

    # Verify that every slave listed in slaves.cfg is associated with exactly
    # one host in slave_hosts_cfg.py.
    for slave_cfg in slaves:
      found_slave = False
      for slave_host_data in slave_hosts_cfg.SLAVE_HOSTS.itervalues():
        for slave_name, _, _ in slave_host_data.slaves:
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
      for slave_name, slave_id, _ in slave_host_data.slaves:
        self.assertFalse(slave_id in known_ids,
            'Slave %s has non-unique id %s' % (slave_name, slave_id))


if __name__ == '__main__':
  unittest.main()

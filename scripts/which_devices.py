#!/usr/bin/env python
# Copyright (c) 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Determine which Android devices should be connected to a host machine."""


import base64
import os
import shlex
import socket
import subprocess
import sys
import urllib2


CHECKOUT_ROOT = os.path.realpath(os.path.join(
    os.path.dirname(__file__), os.pardir))

sys.path.append(os.path.join(CHECKOUT_ROOT, 'site_config'))

import slave_hosts_cfg


USAGE = '''%s: List the Android devices which run on the given host.

If no hostname is provided, list the devices which run on this machine,
and whether or not they are currently connected.

USAGE: %s [hostname]
''' % (__file__, __file__)


def usage():
  """Print a usage message and exit."""
  print >> sys.stderr, USAGE
  sys.exit(1)


def find_slaves(host):
  """Get the list of slaves which run on the given host."""
  slaves = slave_hosts_cfg.get_slave_host_config(host).slaves
  return [s[0] for s in slaves]


def get_device_serials(slaves):
  """Get the serial numbers of the devices which run on the given slaves."""
  ANDROID_DEVICES_URL = ('https://chromium.googlesource.com/chromium/tools/'
                         'build/+/master/scripts/slave/recipe_modules/skia/'
                         'android_devices.py?format=TEXT')
  contents = base64.b64decode(urllib2.urlopen(ANDROID_DEVICES_URL).read())
  env = {}
  exec(contents, env)
  slave_info = env['SLAVE_INFO']
  rv = []
  for slave in slaves:
    if slave_info.get(slave):
      rv.append((slave, slave_info[slave].serial))
  return rv


def get_connected_devices():
  """Return the list of connected Android devices."""
  output = subprocess.check_output(['adb', 'devices']).splitlines()
  rv = []
  for line in output:
    if (line == '' or
        'List of devices attached' in line or
        line.startswith('*') or
        'no permissions' in line):
      continue
    serial, status = shlex.split(line)
    if status == 'device':
      rv.append(serial)
  return rv


def main():
  """List the Android devices that run the given host.

  If the no host is requested, print the devices that run on this machine and
  whether they are connected or not.
  """
  argv = sys.argv[1:]
  local = False
  host = socket.gethostname()
  if len(argv) == 0:
    local = True
  elif len(argv) == 1:
    if argv[0] == '-h' or argv[0] == '--help':
      usage()
    host = argv[0]
  else:
    usage()

  slaves = find_slaves(host)
  device_serials = get_device_serials(slaves)

  connected = {}
  if local:
    serial_list = get_connected_devices()
    for serial in serial_list:
      connected[serial] = True

  print 'Devices for %s:' % host
  for slave, serial in device_serials:
    print '\t%s:\t%s' % (slave, serial),
    if local:
      print '\t%s' % ('connected' if connected.get(serial)
                      else 'not connected'),
    print


if __name__ == '__main__':
  main()

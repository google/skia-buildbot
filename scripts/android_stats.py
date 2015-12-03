#!/usr/bin/env python
# Copyright (c) 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Produce health statistics for all attached Android devices."""


import json
import re
import shlex
import subprocess


def sanitize(string):
  """Return a string which is safe for InfluxDB."""
  return re.sub('\W+', '_', string.strip())


def get_devices():
  """Return a list of attached-and-ready Android devices."""
  devices = []
  for line in subprocess.check_output(['adb', 'devices']).splitlines()[1:]:
    if not line:
      continue
    parts = shlex.split(line)
    if len(parts) != 2:
      continue
    if parts[1] == 'device':
      devices.append(parts[0])
  return devices


def get_device_model(serial):
  """Return the model name for the given device."""
  cmd = ['adb', '-s', serial, 'shell', 'getprop', 'ro.product.model']
  return sanitize(subprocess.check_output(cmd))


def get_battery_stats(serial):
  """Obtain and return a dictionary of battery statistics for the device."""
  cmd = ['adb', '-s', serial, 'shell', 'dumpsys', 'batteryproperties']
  output = subprocess.check_output(cmd)
  parts = re.findall('([a-zA-Z0-9\s]+): (\d+)\s*', output)
  rv = {}
  for k, v in parts:
    rv[sanitize(k)] = int(v)
  return rv


def get_temperature(serial):
  """Obtain and return the temperature of the device."""
  temp_file = '/sys/devices/virtual/thermal/thermal_zone0/temp'
  cmd = ['adb', '-s', serial, 'shell', 'cat', temp_file]
  output = subprocess.check_output(cmd).strip()
  try:
    temp = float(output)
  except Exception:
    return -1
  # Normalize the temperature, assuming it's 9 < t < 100 degrees C.
  while temp > 100.0:
    temp /= 10
  return temp


def get_device_stats(serial):
  """Obtain and return a dictionary of device statistics."""
  return get_device_model(serial), {
    'battery': get_battery_stats(serial),
    'temperature': get_temperature(serial),
  }


def get_all_device_stats():
  """Obtain and return statistics for all attached devices."""
  devices = get_devices()
  stats = {}
  for serial in devices:
    model, device_stats = get_device_stats(serial)
    if not stats.get(model):
      stats[model] = {}
    stats[model][serial] = device_stats
  return stats


if __name__ == '__main__':
  print json.dumps(get_all_device_stats(), sort_keys=True, indent=4)

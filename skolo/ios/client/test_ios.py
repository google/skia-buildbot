#!/usr/bin/python

import time
import unittest

import ios

# Basic unit tests for the ios package. This is intended to be run
# locally (not as part of continuous integration) with the
# libimobiledevice tools installed and an iPad attached.

class IOSDeviceCase(unittest.TestCase):
  def test_get_state(self):
    state = self._get_device().get_state()
    self.assert_(state["ProductType"].lower().find("ipad") != -1)

  def test_reboot(self):
    dev = self._get_device()
    dev.reboot()
    time.sleep(5)
    while True:
      dev = ios.ios_get_device_ids()
      if len(dev) == 1:
        break
      time.sleep(2)
    # Wait an additional 30 seconds to make sure
    # the device is fully rebooted.
    time.sleep(30)

  def test_get_kv_pairs(self):
    val = """ImageSignature[1]:
 0: GD1suZo7maW9nMiDMb+wGAHbug59mPHeMJn/e1BWfjjCDnATA9jWCFg5goyl961sxwhQQttJ8Qj6OuXATQwurfPjQH/zqscAiRzDsk/UQ22/2gtUgVfUGuILtyLeIBvs1u4oF0HJFxb3keV2dqYhK6ATSufLrzZe97k/WSBZPuA="""
    out = ios._get_kv_pairs(val)
    self.assertEqual(1, len(out))
    self.assert_(type(out["ImageSignature"]) is list)
    self.assert_(1, len(out["ImageSignature"]))

    val = """ActivationState: Activated
ActivationStateAcknowledged: true
NonVolatileRAM:
 auto-boot: dHJ1ZQ==
 backlight-level: MTYwMQ==
 boot-args:
SupportedDeviceFamilies[2]:
 0: 1
 1: 2
TelephonyCapability: false"""
    out = ios._get_kv_pairs(val)
    self.assertEqual(5, len(out))
    self.assertEqual(out['NonVolatileRAM'], {
                     'auto-boot': 'dHJ1ZQ==',
                     'backlight-level': 'MTYwMQ==',
                     'boot-args': '',
                     })
    self.assertEqual(out['SupportedDeviceFamilies'], ['1','2'])

  def _get_device(self):
    device = ios.ios_get_devices()
    self.assertEqual(1, len(device))
    device[0].get_ready()
    return device[0]

def main():
  unittest.main()

if __name__ == '__main__':
  main()

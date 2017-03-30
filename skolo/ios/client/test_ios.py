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
            dev = ios.get_device_ids()
            print len(dev)
            if len(dev) == 1:
                break
            time.sleep(2)
        # Wait an additional 30 seconds to make sure
        # the device is fully rebooted.
        time.sleep(30)

    def _get_device(self):
        device = ios.get_devices()
        self.assertEqual(1, len(device))
        return device[0]

def main():
    unittest.main()

if __name__ == "__main__":
    main()
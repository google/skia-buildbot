#!/usr/bin/python

import unittest

import ios

class IOSDeviceCase(unittest.TestCase):
    def test_get_state(self):
        device = ios.get_devices()
        self.assertEqual(1, len(device))
        state = device[0].get_state()
        self.assert_(state["ProductType"].lower().find("ipad") != -1)

def main():
    unittest.main()

if __name__ == "__main__":
    main()
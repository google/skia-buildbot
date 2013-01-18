#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verify that the Android device is attached and functioning properly """


from utils import android_utils, misc
import sys


class AndroidVerifyDevice:
  def _Run(self):
    args = misc.ArgsToDict(sys.argv)
    serial = args['serial']
    android_utils.ADBShell(serial, ['cat', '/system/build.prop'], echo=False)
    print 'Device %s is attached and seems to be working properly.' % serial    


if '__main__' == __name__:
  sys.exit(AndroidVerifyDevice()._Run())

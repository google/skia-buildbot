#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Verify that the Android device is attached and functioning properly """


import os
import sys

BUILDBOT_PATH = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    os.pardir, os.pardir))
sys.path.append(BUILDBOT_PATH)
CHROMIUM_BUILDBOT = os.path.join(BUILDBOT_PATH, 'third_party',
                                'chromium_buildbot')
sys.path.append(os.path.join(CHROMIUM_BUILDBOT, 'scripts'))
sys.path.append(os.path.join(BUILDBOT_PATH, 'common'))

from py.utils import android_utils, misc


class AndroidVerifyDevice:

  # pylint: disable=R0201
  def _Run(self):
    args = misc.ArgsToDict(sys.argv)
    serial = args['serial']
    android_utils.ADBShell(serial, ['cat', '/system/build.prop'], echo=False)
    print 'Device %s is attached and seems to be working properly.' % serial    


if '__main__' == __name__:
  # pylint: disable=W0212
  sys.exit(AndroidVerifyDevice()._Run())

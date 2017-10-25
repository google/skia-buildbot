#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This script should be run on a Swarming bot as part of leasing.skia.org."""

import os
import sys


def main():
  print os.system('hostname')


if __name__ == '__main__':
  sys.exit(main())

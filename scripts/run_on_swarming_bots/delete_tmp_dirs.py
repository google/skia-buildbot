#!/usr/bin/env python
#
# Copyright 2021 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Delete files from the temporary directory on a Swarming bot."""


import os
import sys


if sys.platform == 'win32':
    os.system(r'forfiles /P c:\users\chrome~1\appdata\local\temp '
    r'/M * /C "cmd /c if @isdir==FALSE del @file"')
    os.system(r'forfiles /P c:\users\chrome~1\appdata\local\temp '
    r'/M * /C "cmd /c if @isdir==TRUE rmdir /S /Q @file"')
else:
    os.system(r'rm -rf /tmp/*')

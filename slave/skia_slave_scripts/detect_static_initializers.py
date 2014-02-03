#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Detect static initializers in compiled Skia code. """

from utils import shell_utils
from build_step import BuildStep, BuildStepWarning
import os
import re
import sys
import urllib2


DUMP_STATIC_INITIALIZERS_FILENAME = 'dump-static-initializers.py'
DUMP_STATIC_INITIALIZERS_URL = ('http://src.chromium.org/svn/trunk/src/tools/'
                                + 'linux/' + DUMP_STATIC_INITIALIZERS_FILENAME)


class DetectStaticInitializers(BuildStep):
  def _Run(self):
    # Build the Skia libraries in Release mode.
    os.environ['GYP_DEFINES'] = 'skia_static_initializers=0'
    shell_utils.run(['python', 'gyp_skia'])
    shell_utils.run(['make', 'skia_lib', 'BUILDTYPE=Release', '--jobs'])

    # Obtain the dump-static-initializers script.
    print 'Downloading %s' % DUMP_STATIC_INITIALIZERS_URL
    dl = urllib2.urlopen(DUMP_STATIC_INITIALIZERS_URL)
    with open(DUMP_STATIC_INITIALIZERS_FILENAME, 'wb') as f:
      f.write(dl.read())

    # Run the script over the compiled files.
    results = []
    for built_file_name in os.listdir(os.path.join('out', 'Release')):
      if built_file_name.endswith('.a') or built_file_name.endswith('.so'):
        output = shell_utils.run(['python', DUMP_STATIC_INITIALIZERS_FILENAME,
                                  os.path.join('out', 'Release',
                                               built_file_name)])
        matches = re.search('Found (\d+) static initializers', output)
        if matches:
          num_found = int(matches.groups()[0])
          if num_found:
            results.append((built_file_name, num_found))
    if results:
      print
      print 'Found static initializers:'
      print
      for result in results:
        print '  %s: %d' % result
      print
      # TODO(borenet): Make this an error once we have no static initializers.
      raise BuildStepWarning('Static initializers found!')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DetectStaticInitializers))

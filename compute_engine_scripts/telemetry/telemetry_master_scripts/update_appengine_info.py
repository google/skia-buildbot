#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that updates the skia-telemetry AppEngine WebApp information."""

import os
import subprocess
import sys
import urllib2

import appengine_constants


CHROME_BINARY_LOCATION = (
    '/home/default/storage/chromium-trunk/src/out/Release/chrome')


class UpdateInfo(object):

  def Update(self):
    # Find out when chrome was last built.
    chrome_last_built = int(os.path.getmtime(CHROME_BINARY_LOCATION))
    gce_slaves_cmd = [
        'bash',
        '-c',
        'source ../vm_config.sh && echo $NUM_SLAVES,$NUM_WEBPAGES']

    # Find number of GCE slaves and number of Webpages.
    gce_slaves, num_webpages = subprocess.Popen(
            gce_slaves_cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT).communicate()[0].rstrip().split(',')

    # Find the total number of SKP files currently in Google Storage.
    num_skp_files_cmd = [
        'bash',
        '-c',
        'gsutil ls -l gs://chromium-skia-gm/telemetry/skps/*/*.skp | wc -l']
    num_skp_files = subprocess.Popen(
        num_skp_files_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()

    # Now communicate with the skia-telemetry webapp.
    update_info_url = (
        '%s%s?chrome_last_built=%s&gce_slaves=%s&num_webpages=%s&'
        'num_skp_files=%s' % (
            appengine_constants.SKIA_TELEMETRY_WEBAPP,
            appengine_constants.UPDATE_INFO_SUBPATH,
            chrome_last_built, gce_slaves, num_webpages, num_skp_files))
    os.system('wget -o /dev/null "%s"' % update_info_url)


if '__main__' == __name__:
  sys.exit(UpdateInfo().Update())


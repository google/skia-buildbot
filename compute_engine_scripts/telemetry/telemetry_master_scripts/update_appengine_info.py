#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that updates the skia-telemetry AppEngine WebApp information."""

import os
import re
import subprocess
import sys
import time

import appengine_constants


CHROME_SRC = '/home/default/storage/chromium-trunk/src/'
CHROME_BINARY_LOCATION = CHROME_SRC + 'out/Release/chrome'


class UpdateInfo(object):

  def Update(self):
    # Find out when chrome was last built.
    chrome_last_built = int(os.path.getmtime(CHROME_BINARY_LOCATION))
    print 'chrome_last_built: %s' % chrome_last_built

    # Find number of GCE slaves and number of webpages per pageset.
    gce_slaves_cmd = [
        'bash',
        '-c',
        'source ../vm_config.sh && echo $NUM_SLAVES,$MAX_WEBPAGES_PER_PAGESET']
    gce_slaves, num_webpages_per_pageset = subprocess.Popen(
            gce_slaves_cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT).communicate()[0].rstrip().split(',')
    print 'gce_slaves: %s' % gce_slaves
    print 'num_webpages_per_pageset: %s' % num_webpages_per_pageset

    # Find the total number of archived webpages currently in Google Storage.
    num_archives_cmd = [
        'bash',
        '-c',
        'gsutil ls -l gs://chromium-skia-gm/telemetry/webpages_archive/*/All/*.json | wc -l']
    num_archives = subprocess.Popen(
        num_archives_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()
    num_webpages = int(num_webpages_per_pageset) * int(num_archives)
    print 'num_webpages: %s' % num_webpages

    # Find the total number of SKP files currently in Google Storage.
    num_skp_files_cmd = [
        'bash',
        '-c',
        'gsutil ls -l gs://chromium-skia-gm/telemetry/skps/*/All/*.skp | wc -l']
    num_skp_files = subprocess.Popen(
        num_skp_files_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()
    print 'num_skp_files: %s' % num_skp_files

    # Get Chromium and Skia revisions.
    old_cwd = os.getcwd()
    os.chdir(CHROME_SRC)
    version_cmd = [
        'svnversion' ,
        '.']
    # Sometimes 'svnversion .' returns the wrong output and needs to be retried.
    for _ in range(10):
      svnversion_output = subprocess.Popen(
          version_cmd,
          stdout=subprocess.PIPE,
          stderr=subprocess.STDOUT).communicate()[0].rstrip()
      print 'svnversion output: %s' % svnversion_output
      match_obj = re.search(
          r'([0-9]+):([0-9]+).*', svnversion_output, re.M|re.I)
      try:
        skia_rev = match_obj.group(1)
        if skia_rev <= 1:
          raise Exception('Got an invalid skia revision!')
        chromium_rev = match_obj.group(2)
      except Exception as e:
        print e
        skia_rev = '0'
        chromium_rev = '0'
        # There was an exception retry the command after sleeping.
        time.sleep(5)
        continue
      break

    print 'skia_rev: %s' % skia_rev
    print 'chromium_rev: %s' % chromium_rev
    os.chdir(old_cwd)
    # Now communicate with the skia-telemetry webapp.
    update_info_url = '%s%s' % (
        appengine_constants.SKIA_TELEMETRY_WEBAPP.replace('http', 'https'),
        appengine_constants.UPDATE_INFO_SUBPATH)
    password = open('appengine_password.txt').read().rstrip()
    update_info_post_data = (
        'chrome_last_built=%s&gce_slaves=%s&num_webpages=%s&'
        'num_skp_files=%s&chromium_rev=%s&skia_rev=%s&password=%s&'
        'num_webpages_per_pageset=%s' % (
            chrome_last_built, gce_slaves, num_webpages, num_skp_files,
            chromium_rev, skia_rev, password, num_webpages_per_pageset))
    os.system('wget --post-data "%s" "%s" -O /dev/null' % (
        update_info_post_data, update_info_url))


if '__main__' == __name__:
  sys.exit(UpdateInfo().Update())


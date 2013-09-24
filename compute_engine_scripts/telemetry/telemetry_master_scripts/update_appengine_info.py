#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that updates the skia-telemetry AppEngine WebApp information."""

import os
import subprocess
import sys

import appengine_constants


CHROME_SRC = '/home/default/storage/chromium-trunk/src/'
SKIA_SRC = CHROME_SRC + 'third_party/skia/'
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
        'gsutil ls -l gs://chromium-skia-gm/telemetry/webpages_archive' \
        '/*/All/*.json | wc -l']
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
    version_cmd = [
        'git',
        'rev-parse',
        'HEAD']
    os.chdir(CHROME_SRC)
    chromium_rev = subprocess.Popen(
        version_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()
    os.chdir(SKIA_SRC)
    skia_rev = subprocess.Popen(
        version_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()
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


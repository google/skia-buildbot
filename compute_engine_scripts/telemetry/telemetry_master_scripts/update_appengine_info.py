#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that updates the skia-telemetry AppEngine WebApp information."""

import os
import re
import subprocess
import sys

import appengine_constants


CHROME_SRC = '/home/default/storage/chromium-trunk/src/'
CHROME_BINARY_LOCATION = CHROME_SRC + 'out/Release/chrome'


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

    # Get Chromium and Skia revisions.
    old_cwd = os.getcwd()
    os.chdir(CHROME_SRC)
    version_cmd = [
        'svnversion' ,
        '.']
    svnversion_output = subprocess.Popen(
        version_cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT).communicate()[0].rstrip()
    match_obj = re.search(r'([0-9]+):([0-9]+).*', svnversion_output, re.M|re.I)
    skia_rev = match_obj.group(1)
    chromium_rev = match_obj.group(2)
    os.chdir(old_cwd)

    # Now communicate with the skia-telemetry webapp.
    update_info_url = '%s%s' % (
        appengine_constants.SKIA_TELEMETRY_WEBAPP,
        appengine_constants.UPDATE_INFO_SUBPATH)
    password = open('appengine_password.txt').read().rstrip()
    update_info_post_data = (
        'chrome_last_built=%s&gce_slaves=%s&num_webpages=%s&'
        'num_skp_files=%s&chromium_rev=%s&skia_rev=%s&password=%s' % (
            chrome_last_built, gce_slaves, num_webpages, num_skp_files,
            chromium_rev, skia_rev, password))
    os.system('wget --post-data "%s" "%s" -O /dev/null' % (
        update_info_post_data, update_info_url))


if '__main__' == __name__:
  sys.exit(UpdateInfo().Update())


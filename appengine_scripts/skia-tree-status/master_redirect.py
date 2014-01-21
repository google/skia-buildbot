# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Page that redirects to the currently running Skia Buildbot master."""

import httplib
import logging
import urllib2

import base_page


# Buildbot master static IP addresses.
# For faster response time these IPs should be switched when GCE instances are
# migrated during PCRs.
MASTER_IPS = ('108.170.217.252', '173.255.115.253')
# Buildbot master ports.
MASTER_CONSOLE_PORT = 10117
MASTER_REPO_SERVING_PORT = 8000
# Special subparts of master.
MASTER_CONSOLE_SUBPART = 'console'


def _get_destination_url(port=None, subparts=None):
  for master_ip in MASTER_IPS:
    destination_url = 'http://%s' % master_ip
    if port:
      destination_url += ':%s' % port
    if subparts:
      destination_url += '/%s' % subparts

    logging.error('Trying out %s' % master_ip)
    try:
      urllib2.urlopen(destination_url, timeout=10).getcode()
      return destination_url
    except httplib.HTTPException, e:
      logging.warning(e)

  error_msg = ('The buildbot master could not be contacted at either of %s' %
               str(MASTER_IPS))
  logging.error(error_msg)
  raise Exception(error_msg)


class MasterConsolePage(base_page.BasePage):
  """Redirects to the console page of the currently running buildbot master."""

  def get(self):
    destination_url = _get_destination_url(port=MASTER_CONSOLE_PORT,
                                           subparts=MASTER_CONSOLE_SUBPART)
    self.redirect(destination_url, True)


class MasterRepoServingPage(base_page.BasePage):
  """Redirects to the currently running buildbot master."""

  def get(self, slug, *args):
    destination_url = _get_destination_url(port=MASTER_REPO_SERVING_PORT,
                                           subparts=slug)
    self.redirect(destination_url, True)


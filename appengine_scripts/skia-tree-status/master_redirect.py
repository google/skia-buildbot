# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Pages that redirect to the currently running Skia Buildbot master."""

import httplib
import logging
import urllib2

import base_page


# Buildbot master static IP addresses.
# For faster response time these IPs should be switched when GCE instances are
# migrated during PCRs.
MASTER_IPS = ('108.170.217.252', '173.255.115.253')
# Map of service type to a tuple containing the (port, subpart, protocol).
SERVICE_TYPE_TO_INFO = {
    'buildbots': (10117, '', 'http'),
    'repo-serving': (8000, '', 'http'),
}


def _get_destination_url(service_type, slug):
  (port, subpart, protocol) = SERVICE_TYPE_TO_INFO[service_type]
  for master_ip in MASTER_IPS:
    destination_url = 'http://%s' % master_ip
    if port:
      destination_url += ':%s' % port

    try:
      # Test if the destination URL is up.
      urllib2.urlopen(destination_url, timeout=15).getcode()
      if protocol:
        # If the protocol has been specified then replace the destination URL
        # with it. We do not use the protocol for the initial URL check, it
        # should always be HTTP.
        destination_url = destination_url.replace('http', protocol)
        # Add the subpart and slug to the destination URL.
        if subpart:
          destination_url += '/%s' % subpart
        if slug:
          destination_url += '/%s' % slug
      return destination_url
    except httplib.HTTPException, e:
      logging.warning(e)

  error_msg = ('The buildbot master could not be contacted at either of %s' %
               str(MASTER_IPS))
  logging.error(error_msg)
  raise Exception(error_msg)


class MasterConsolePage(base_page.BasePage):
  """Redirects to the console page of the currently running buildbot master."""
  def get(self, slug, *args):
    destination_url = _get_destination_url(service_type='buildbots',
                                           slug=slug)
    self.redirect(destination_url, True)


class MasterRepoServingPage(base_page.BasePage):
  """Redirects to the currently running buildbot master."""
  def get(self, slug, *args):
    destination_url = _get_destination_url(service_type='repo-serving',
                                           slug=slug)
    self.redirect(destination_url, True)


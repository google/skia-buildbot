# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Pages that redirect to the currently running Skia Buildbot master."""

import httplib
import logging
import urllib2

import base_page


# Static IP addresses of our various services.
# For faster response time these IPs should be reordered when GCE instances are
# migrated during PCRs (preferred server should be listed first).
BUILDBOT_MASTER_IPS =         ['108.170.217.252', '173.255.115.253']
FYI_BUILDBOT_MASTER_IPS =     ['108.170.219.160', '108.170.219.161']
ANDROID_BUILDBOT_MASTER_IPS = ['108.170.219.162', '108.170.219.163']
COMPILE_BUILDBOT_MASTER_IPS = ['108.170.219.164', '108.170.219.165']
REBASELINE_SERVER_IPS =       ['108.170.217.246']

# Map of service type to (list_of_possible_ip_addrs, port, subpart, protocol).
# The list_of_possible_ip_addrs, port, and protocol must be specified;
# subpart can be an empty string if not needed.
SERVICE_TYPE_TO_INFO = {
    'buildbots':          (        BUILDBOT_MASTER_IPS, 10117, '', 'http'),
    'fyi-buildbots':      (    FYI_BUILDBOT_MASTER_IPS, 10117, '', 'http'),
    'android-buildbots':  (ANDROID_BUILDBOT_MASTER_IPS, 10117, '', 'http'),
    'compile-buildbots':  (COMPILE_BUILDBOT_MASTER_IPS, 10117, '', 'http'),
    'rebaseline-server':  (      REBASELINE_SERVER_IPS, 10117, '', 'http'),
    'repo-serving':       (        BUILDBOT_MASTER_IPS,  8000, '', 'http'),
}
# Timeout to use in urlopen when determining which IP is up.
URLOPEN_TIMEOUT = 15


# TODO(rmistry): Add unittests for this function.
def _get_destination_url(service_type, slug):
  """Returns a complete destination URL from the service type and slug.

  This function first determines which master from the possible_ips list is up.
  It then constructs a URL using the service type's port, subpart and protocol.
  The specified slug is then appended to this url.

  Args:
    service_type: string; Which service_type to look up in SERVICE_TYPE_TO_INFO.
        If None, use the first path element within slug as the service_type.
    slug: string; Path elements that appengine handed us to deal with--we append
        this to the server-specific URL we create here.

  Returns:
    A destination URL.
  """
  if service_type:
    remaining_slug = slug
  elif '/' in slug:
    service_type, remaining_slug = slug.split('/', 1)
  else:
    service_type = slug
    remaining_slug = None

  (possible_ips, port, subpart, protocol) = SERVICE_TYPE_TO_INFO[service_type]
  for ipaddr in possible_ips:
    # The test URL is used for validating that the IP + Port combination is up.
    test_url = 'http://%s:%s' % (ipaddr, port)
    try:
      urllib2.urlopen(test_url, timeout=URLOPEN_TIMEOUT).getcode()
      # Now that we verified that the test URL is up use the service's protocol
      # to construct the destination URL.
      destination_url = '%s://%s:%s' % (protocol, ipaddr, port)
      # Add the subpart and remaining_slug to the destination URL.
      if subpart:
        destination_url += '/%s' % subpart
      if remaining_slug:
        destination_url += '/%s' % remaining_slug
      return destination_url
    except httplib.HTTPException, e:
      logging.warning(e)

  error_msg = ('The buildbot master could not be contacted at any of %s' %
               str(possible_ips))
  logging.error(error_msg)
  raise Exception(error_msg)


class GenericRedirectionPage(base_page.BasePage):
  """Generic redirector which lets _get_destination_url() figure out the
  service_type."""
  def get(self, slug, *args):
    destination_url = _get_destination_url(service_type=None, slug=slug)
    self.redirect(destination_url, True)


# TODO(epoger): Delete in favor of GenericRedirectionPage?
# See https://codereview.chromium.org/145583011/#msg2
class MasterBuildbotPage(base_page.BasePage):
  """Redirects to the buildbot page of the currently running buildbot master."""
  def get(self, slug, *args):
    destination_url = _get_destination_url(service_type='buildbots',
                                           slug=slug)
    self.redirect(destination_url, True)


# TODO(epoger): Delete in favor of GenericRedirectionPage?
# See https://codereview.chromium.org/145583011/#msg2
class MasterRepoServingPage(base_page.BasePage):
  """Redirects to the repo page of the currently running buildbot master."""
  def get(self, slug, *args):
    destination_url = _get_destination_url(service_type='repo-serving',
                                           slug=slug)
    self.redirect(destination_url, True)

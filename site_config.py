# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Subclass of chromium_buildbot/site_config/config.py to override with
locally appropriate values.
"""

import socket

# import base class from third_party/chromium_buildbot/site_config/
import config

class Master(config.Master):
  googlecode_revlinktmpl = 'http://code.google.com/p/%s/source/browse?r=%s'

  # domains to which we will send blame emails
  permitted_domains = ['google.com', 'chromium.org']

  @staticmethod
  def GetBotPassword():
    return 'epoger-temp-password'

  class Skia(object):
    project_name = 'Skia'
    project_url = 'http://skia.googlecode.com'
    # For now, this is epoger's Ubiquity instance.
    master_host = 'wpgntat-ubiq141.hot.corp.google.com'
    is_production_host = socket.getfqdn() == master_host
    master_port = 8041
    slave_port = 8141
    master_port_alt = 8241
    tree_closing_notification_recipients = []
    from_address = 'skia-buildbot@google.com'
    buildbot_url = 'http://%s:%d/' % (master_host, master_port)

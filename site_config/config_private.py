# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# These buildbot configurations are "private" in the sense that they are
# specific to Skia buildbots (not shared by other Chromium buildbots).
# But this file is stored within a public SVN repository, so don't put any
# secrets in here.

# system-level imports
import socket

# import base class from third_party/chromium_buildbot/site_config/
import config_default

class Master(config_default.Master):
  googlecode_revlinktmpl = 'http://code.google.com/p/%s/source/browse?r=%s'
  bot_password = 'epoger-temp-password'
  default_clobber = True

  # domains to which we will send blame emails
  permitted_domains = ['google.com', 'chromium.org']

  class Skia(object):
    project_name = 'Skia'
    project_url = 'http://skia.googlecode.com'
    # The master host runs in Google Compute Engine.
    master_host = '70.32.156.51'
    is_production_host = socket.getfqdn() == 'skia-master.c.skia-buildbots.google.com.internal'
    master_port = 10115
    slave_port = 10116
    master_port_alt = 10117
    tree_closing_notification_recipients = []
    from_address = 'skia-buildbot@pogerlabs.com'
    buildbot_url = 'http://%s:%d/' % (master_host, master_port)

class Archive(config_default.Archive):
    bogus_var = 'bogus_value'

class Distributed(config_default.Distributed):
    bogus_var = 'bogus_value'

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

  @staticmethod
  def GetBotPassword():
    return 'epoger-temp-password'

  # epoger: for now, copied from build_internal/site_config/config_private.py
  class _Master3(object):
    """Client master."""
    master_host = 'master3.golo.chromium.org'
    is_production_host = socket.getfqdn() == 'master3.golo.chromium.org'
    tree_closing_notification_recipients = []
    from_address = 'buildbot@chromium.org'

  # epoger: for now, copied from build_internal/site_config/config_private.py
  class Skia(_Master3):
    project_name = 'Skia'
    master_port = 8041
    slave_port = 8141
    master_port_alt = 8241
    project_url = 'http://skia.googlecode.com'

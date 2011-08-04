# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import socket

# import base class from third_party/chromium_buildbot/site_config/
import config_default

class Master(config_default.Master):
  googlecode_revlinktmpl = 'http://code.google.com/p/%s/source/browse?r=%s'
  bot_password = 'epoger-temp-password'

  # domains to which we will send blame emails
  permitted_domains = []

  class _Base(object):
    # If set to True, the master will do nasty stuff like closing the tree,
    # sending emails or other similar behaviors. Don't change this value unless
    # you modified the other settings extensively.
    is_production_host = False
    # Master address. You should probably copy this file in another svn repo
    # so you can override this value on both the slaves and the master.
    master_host = 'localhost'
    # Additional email addresses to send gatekeeper (automatic tree closage)
    # notifications. Unnecessary for experimental masters and try servers.
    tree_closing_notification_recipients = []
    # 'from:' field for emails sent from the server.
    from_address = 'nobody@example.com'
    # Code review site to upload results. You should setup your own Rietveld
    # instance with the code at
    # http://code.google.com/p/rietveld/source/browse/#svn/branches/chromium
    # and put a url looking like this:
    # 'http://codereview.chromium.org/%d/upload_build_result/%d'
    # You can host your own private rietveld instance on Django, see
    # http://code.google.com/p/google-app-engine-django and
    # http://code.google.com/appengine/articles/pure_django.html
    code_review_site = None

    # For the following values, they are used only if non-0. Do not set them
    # here, set them in the actual master configuration class.

    # Used for the waterfall URL and the waterfall's WebStatus object.
    master_port = 0
    # Which port slaves use to connect to the master.
    slave_port = 0
    # The alternate read-only page. Optional.
    master_port_alt = 0
    # HTTP port for try jobs.
    try_job_port = 0

  class _ChromiumBase(_Base):
    # Tree status urls. You should fork the code from tools/chromium-status/ and
    # setup your own AppEngine instance (or use directly Djando to create a
    # local instance).
    # Defaulting urls that are used to POST data to 'localhost' so a local dev
    # server can be used for testing and to make sure nobody updates the tree
    # status by error!
    #
    # This url is used for HttpStatusPush:
    base_app_url = 'http://localhost:8080'
    # HTTP url that should return 0 or 1, depending if the tree is open or
    # closed. It is also used as POST to update the tree status.
    tree_status_url = base_app_url + '/status'
    # Used by LKGR to POST data.
    store_revisions_url = base_app_url + '/revisions'
    # Used by the try server to sync to the last known good revision:
    last_good_url = 'http://chromium-status.appspot.com/lkgr'

  class ChromiumFYI(_ChromiumBase):
    project_name = 'Chromium FYI'
    master_port = 9016
    slave_port = 9017
    master_port_alt = 9019
    master_host = 'c128.i.corp.google.com'
    is_production_host = False

class Installer(config_default.Installer):
    bogus_var = 'bogus_value'

class Archive(config_default.Archive):
    bogus_var = 'bogus_value'

class Distributed(config_default.Distributed):
    bogus_var = 'bogus_value'

# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# These buildbot configurations are "private" in the sense that they are
# specific to Skia buildbots (not shared by other Chromium buildbots).
# But this file is stored within a public SVN repository, so don't put any
# secrets in here.


import socket
import skia_vars

# import base class from third_party/chromium_buildbot/site_config/
import config_default


CODE_REVIEW_SITE = skia_vars.GetGlobalVariable('code_review_site')

# On startup, the build master validates the bot configuration against a known
# expectation.  If this variable is set to true (eg. in a buildbot self-test),
# the master will fail to start up if validation fails.
#
# For more information: https://code.google.com/p/skia/issues/detail?id=1289
die_on_validation_failure = False

# Skia's Google Compute Engine instances.
# The public master which is visible to everyone.
SKIA_PUBLIC_MASTER_INTERNAL_FQDN = skia_vars.GetGlobalVariable(
    'public_master_internal_fqdn')
# The private master which is visible only to Google corp.
SKIA_PRIVATE_MASTER_INTERNAL_FQDN = skia_vars.GetGlobalVariable(
    'private_master_internal_fqdn')
AUTOGEN_SVN_BASEURL = skia_vars.GetGlobalVariable('autogen_svn_url')
SKIA_REVLINKTMPL = skia_vars.GetGlobalVariable('revlinktmpl')
SKIA_SVN_BASEURL = skia_vars.GetGlobalVariable('skia_svn_url')
TRY_SVN_BASEURL = skia_vars.GetGlobalVariable('try_svn_url')

class Master(config_default.Master):
  googlecode_revlinktmpl = 'http://code.google.com/p/%s/source/browse?r=%s'
  bot_password = 'epoger-temp-password'
  default_clobber = False

  # SMTP configurations.
  smtp_server = skia_vars.GetGlobalVariable('gce_smtp_server')
  smtp_port = skia_vars.GetGlobalVariable('gce_smtp_port')
  smtp_use_tls = skia_vars.GetGlobalVariable('gce_smtp_use_tls')
  smtp_user = skia_vars.GetGlobalVariable('gce_smtp_user')
  smtp_password = open('.skia_buildbots_password').readline().strip()

  # domains to which we will send blame emails
  permitted_domains = ['google.com', 'chromium.org']

  class Skia(object):
    project_name = 'Skia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    # The master host runs in Google Compute Engine.
    master_host = skia_vars.GetGlobalVariable('public_master_host')
    is_production_host = socket.getfqdn() == SKIA_PUBLIC_MASTER_INTERNAL_FQDN
    master_port = skia_vars.GetGlobalVariable('public_internal_port')
    slave_port = skia_vars.GetGlobalVariable('public_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('public_external_port')
    tree_closing_notification_recipients = ['skia-commit@googlegroups.com']
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = True
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')
    tree_status_url = skia_vars.GetGlobalVariable('tree_status_url')

  class PrivateSkia(object):
    project_name = 'PrivateSkia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    # The private master host runs in Google Compute Engine.
    master_host = skia_vars.GetGlobalVariable('private_master_host')
    is_production_host = socket.getfqdn() == SKIA_PRIVATE_MASTER_INTERNAL_FQDN
    master_port = skia_vars.GetGlobalVariable('private_internal_port')
    slave_port = skia_vars.GetGlobalVariable('private_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('private_external_port')
    tree_closing_notification_recipients = []
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = False
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')
  

class Archive(config_default.Archive):
  bogus_var = 'bogus_value'


class Distributed(config_default.Distributed):
  bogus_var = 'bogus_value'

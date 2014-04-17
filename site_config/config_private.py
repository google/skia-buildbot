# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# These buildbot configurations are "private" in the sense that they are
# specific to Skia buildbots (not shared by other Chromium buildbots).
# But this file is stored within a public SVN repository, so don't put any
# secrets in here.


import socket
import skia_vars
import sys

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

# The FYI master.
SKIA_FYI_MASTER_INTERNAL_FQDN = skia_vars.GetGlobalVariable(
    'fyi_master_internal_fqdn')

# The Android master.
SKIA_ANDROID_MASTER_INTERNAL_FQDN = skia_vars.GetGlobalVariable(
    'android_master_internal_fqdn')

# The Compile master.
SKIA_COMPILE_MASTER_INTERNAL_FQDN = skia_vars.GetGlobalVariable(
    'compile_master_internal_fqdn')

AUTOGEN_SVN_BASEURL = skia_vars.GetGlobalVariable('autogen_svn_url')
SKIA_GIT_URL = skia_vars.GetGlobalVariable('skia_git_url')
TRY_SVN_BASEURL = skia_vars.GetGlobalVariable('try_svn_url')

# This env variable contains a comma-separated list of build steps to skip.
SKIPSTEPS_ENVIRONMENT_VARIABLE = 'TESTING_SKIPSTEPS'


# Currently-active master instance; this is set in Master.set_active_master.
_ACTIVE_MASTER = None


class Master(config_default.Master):
  googlecode_revlinktmpl = 'http://code.google.com/p/%s/source/browse?r=%s'
  bot_password = 'epoger-temp-password'
  default_clobber = False

  # SMTP configurations.
  smtp_server = skia_vars.GetGlobalVariable('gce_smtp_server')
  smtp_port = skia_vars.GetGlobalVariable('gce_smtp_port')
  smtp_use_tls = skia_vars.GetGlobalVariable('gce_smtp_use_tls')
  smtp_user = skia_vars.GetGlobalVariable('gce_smtp_user')

  # domains to which we will send blame emails
  permitted_domains = ['google.com', 'chromium.org']

  class Skia(object):
    project_name = 'Skia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    master_host = skia_vars.GetGlobalVariable('public_master_host')
    is_production_host = socket.getfqdn() == SKIA_PUBLIC_MASTER_INTERNAL_FQDN
    _skip_render_results_upload = False
    _skip_bench_results_upload = False
    master_port = skia_vars.GetGlobalVariable('public_internal_port')
    slave_port = skia_vars.GetGlobalVariable('public_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('public_external_port')
    tree_closing_notification_recipients = ['skia-commit@googlegroups.com']
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = True
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')
    tree_status_url = skia_vars.GetGlobalVariable('tree_status_url')

    @property
    def do_upload_render_results(self):
      return self.is_production_host and not self._skip_render_results_upload

    @property
    def do_upload_bench_results(self):
      return self.is_production_host and not self._skip_bench_results_upload

    def create_schedulers_and_builders(self, cfg):
      """Create the Schedulers and Builders.

      Args:
          cfg: dict; configuration dict for the build master.
      """
      # This import needs to happen inside this function because modules
      # imported by master_builders_cfg import this module.
      import master_builders_cfg
      master_builders_cfg.create_schedulers_and_builders(
          sys.modules[__name__],
          self,
          cfg,
          master_builders_cfg.setup_all_builders)

  class PrivateSkia(Skia):
    project_name = 'PrivateSkia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    master_host = skia_vars.GetGlobalVariable('private_master_host')
    is_production_host = socket.getfqdn() == SKIA_PRIVATE_MASTER_INTERNAL_FQDN
    _skip_render_results_upload = False
    # Don't upload bench results on the private master, since we don't yet have
    # a private destination for them.
    _skip_bench_results_upload = True
    master_port = skia_vars.GetGlobalVariable('private_internal_port')
    slave_port = skia_vars.GetGlobalVariable('private_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('private_external_port')
    tree_closing_notification_recipients = []
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = False
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')

    def create_schedulers_and_builders(self, cfg):
      """Create the Schedulers and Builders.

      Args:
          cfg: dict; configuration dict for the build master.
      """
      # These imports needs to happen inside this function because modules
      # imported by master_builders_cfg import this module.
      import master_builders_cfg
      import master_private_builders_cfg
      master_builders_cfg.create_schedulers_and_builders(
          sys.modules[__name__],
          self,
          cfg,
          master_private_builders_cfg.setup_all_builders)

  class FYISkia(Skia):
    project_name = 'FYISkia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    master_host = skia_vars.GetGlobalVariable('fyi_master_host')
    is_production_host = socket.getfqdn() == SKIA_FYI_MASTER_INTERNAL_FQDN
    _skip_render_results_upload = False
    _skip_bench_results_upload = False
    master_port = skia_vars.GetGlobalVariable('fyi_internal_port')
    slave_port = skia_vars.GetGlobalVariable('fyi_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('fyi_external_port')
    tree_closing_notification_recipients = []
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = True
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')

    def create_schedulers_and_builders(self, cfg):
      """Create the Schedulers and Builders.

      Args:
          cfg: dict; configuration dict for the build master.
      """
      # These imports needs to happen inside this function because modules
      # imported by master_builders_cfg import this module.
      import master_builders_cfg
      import master_fyi_builders_cfg
      master_builders_cfg.create_schedulers_and_builders(
          sys.modules[__name__],
          self,
          cfg,
          master_fyi_builders_cfg.setup_all_builders)

  class AndroidSkia(Skia):
    project_name = 'AndroidSkia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    master_host = skia_vars.GetGlobalVariable('android_master_host')
    is_production_host = socket.getfqdn() == SKIA_ANDROID_MASTER_INTERNAL_FQDN
    _skip_render_results_upload = False
    _skip_bench_results_upload = False
    master_port = skia_vars.GetGlobalVariable('android_internal_port')
    slave_port = skia_vars.GetGlobalVariable('android_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('android_external_port')
    tree_closing_notification_recipients = []
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = True
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')

    def create_schedulers_and_builders(self, cfg):
      """Create the Schedulers and Builders.

      Args:
          cfg: dict; configuration dict for the build master.
      """
      # These imports needs to happen inside this function because modules
      # imported by master_builders_cfg import this module.
      import master_builders_cfg
      import master_android_builders_cfg
      master_builders_cfg.create_schedulers_and_builders(
          sys.modules[__name__],
          self,
          cfg,
          master_android_builders_cfg.setup_all_builders)

  class CompileSkia(Skia):
    project_name = 'CompileSkia'
    project_url = skia_vars.GetGlobalVariable('project_url')
    master_host = skia_vars.GetGlobalVariable('compile_master_host')
    is_production_host = socket.getfqdn() == SKIA_COMPILE_MASTER_INTERNAL_FQDN
    _skip_render_results_upload = False
    _skip_bench_results_upload = False
    master_port = skia_vars.GetGlobalVariable('compile_internal_port')
    slave_port = skia_vars.GetGlobalVariable('compile_slave_port')
    master_port_alt = skia_vars.GetGlobalVariable('compile_external_port')
    tree_closing_notification_recipients = []
    from_address = skia_vars.GetGlobalVariable('gce_smtp_user')
    is_publicly_visible = True
    code_review_site = \
        skia_vars.GetGlobalVariable('code_review_status_listener')

    def create_schedulers_and_builders(self, cfg):
      """Create the Schedulers and Builders.

      Args:
          cfg: dict; configuration dict for the build master.
      """
      # These imports needs to happen inside this function because modules
      # imported by master_builders_cfg import this module.
      import master_builders_cfg
      import master_compile_builders_cfg
      master_builders_cfg.create_schedulers_and_builders(
          sys.modules[__name__],
          self,
          cfg,
          master_compile_builders_cfg.setup_all_builders)

  # List of the valid master classes.
  valid_masters = [Skia, PrivateSkia, FYISkia, AndroidSkia, CompileSkia]

  @staticmethod
  def set_active_master(master_name):
    """Sets the master with the given name as active and returns its instance.

    Args:
        master_name: string; name of the desired build master.
    """
    global _ACTIVE_MASTER
    master = Master.get(master_name)
    if master:
      _ACTIVE_MASTER = master()
      return _ACTIVE_MASTER
    raise Exception('Invalid master: %s' % master_name)

  @staticmethod
  def get(master_name):
    """Return the master with the given name or None if no such master exists.

    Args:
        master_name: string; name of the desired build master.
    """
    for master in Master.valid_masters:
      if master_name == master.__name__:
        return master
    return None

  @staticmethod
  def get_active_master():
    """Returns the instance of the active build master."""
    return _ACTIVE_MASTER

class Archive(config_default.Archive):
  bogus_var = 'bogus_value'


class Distributed(config_default.Distributed):
  bogus_var = 'bogus_value'

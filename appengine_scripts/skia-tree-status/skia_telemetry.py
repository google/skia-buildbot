# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Skia Telemetry pages."""


import datetime
import json
import urllib2

from google.appengine.ext import db

from base_page import BasePage
import utils


FETCH_LIMIT = 50
PAGINATION_LIMIT = 10

TELEMETRY_ADMINS = (
    'rmistry@google.com',
)

PDF_ADMINS = (
    'edisonn@google.com',
    'rmistry@google.com',
)

PAGESET_TYPES = {
    'All': 'Top 1M (with desktop user-agent)',
    '10k': 'Top 10k (with desktop user-agent)',
    'Mobile10k': 'Top 10k (with mobile user-agent)',
    'IndexSample10k': 'IndexSample 10k (with mobile user-agent)'
}

# Constants for ChromiumTryPage.
CHROMIUM_TRY_SUPPORTED_BENCHMARKS = (
    'rasterize_and_record_micro',
    'pixeldiffs',
    'smoothness',
    'loading_trace',
    'loading_profile'
)

# LKGR urls.
CHROMIUM_LKGR_URL = 'http://chromium-status.appspot.com/git-lkgr'
SKIA_LKGR_URL = 'http://skia-tree-status.appspot.com/git-lkgr'


class BaseTelemetryModel(db.Model):
  """Base class for Telemetry Datamodels."""

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model.

    Must be implemented by subclasses.
    """
    raise NotImplementedError('Cannot directly use BaseTelemetryModel.')

  @classmethod
  def get_pending_tasks(cls):
    """Returns all pending tasks."""
    return (cls.all()
               .filter('completed_time =', None)
               .order('requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def add_pending_tasks_in_json(cls, l):
    """Adds all pending tasks in their JSON format to the specified list."""
    db_obj = cls.get_pending_tasks()
    if db_obj:
      for db_model in db_obj:
        l.append(db_model.get_json_repr())

  @classmethod
  def add_oldest_pending_task(cls, l):
    """Adds the oldest pending task to the specified list."""
    db_obj = cls.get_pending_tasks()
    if db_obj:
      l.append(db_obj[0])


class ChromiumBuilds(BaseTelemetryModel):
  """Datamodel for Chromium builds."""
  chromium_rev = db.StringProperty(required=True)
  skia_rev = db.StringProperty(required=True)
  username = db.StringProperty(required=True)
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  build_log_link = db.LinkProperty()
  chromium_rev_date = db.DateTimeProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'ChromiumBuildTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'chromium_rev': self.chromium_rev,
            'skia_rev': self.skia_rev,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_all_chromium_builds(cls):
    return (cls.all()
               .order('-chromium_rev_date')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_all_completed_chromium_builds(cls):
    return (cls.all()
               .filter('completed_time !=', None)
               .order('-completed_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_chromium_build_with_key(cls, key):
    return db.GqlQuery(
        'SELECT * FROM ChromiumBuilds WHERE __key__ = Key(\'ChromiumBuilds\','
        ' %s);' % key)

  @classmethod
  def get_chromium_build_with_revs(cls, chromium_rev, skia_rev):
    return db.GqlQuery(
        'SELECT * FROM ChromiumBuilds WHERE chromium_rev=\'%s\' '
        'AND skia_rev=\'%s\';' % (chromium_rev, skia_rev))

  @classmethod
  def delete_chromium_build(cls, key):
    chromium_builds = cls.get_chromium_build_with_key(key)
    if chromium_builds.count():
      chromium_builds[0].delete()


class TelemetryInfo(db.Model):
  """Contains a single row of Skia Telemetry data."""
  chrome_last_built = db.DateTimeProperty(required=True)
  gce_slaves = db.IntegerProperty(required=True)
  num_webpages = db.IntegerProperty(required=True)
  num_webpages_per_pageset = db.IntegerProperty(required=True)
  num_skp_files = db.IntegerProperty(required=True)
  last_updated = db.DateTimeProperty(required=True)
  skia_rev = db.StringProperty(required=True)
  chromium_rev = db.StringProperty(required=True)
  pagesets_source = db.LinkProperty(
      required=True,
      default='https://storage.cloud.google.com/chromium-skia-gm/telemetry/csv'
              '/top-1m.csv')
  framework_msg = db.StringProperty()

  @classmethod
  def get_telemetry_info(cls):
    return cls.all().fetch(limit=1)[0]


def tasks_counter(cls):
  return cls.all(keys_only=True).count()


class AdminTasks(BaseTelemetryModel):
  """Data model for Admin tasks."""
  username = db.StringProperty(required=True)
  task_name = db.StringProperty(required=True)
  requested_time = db.DateTimeProperty(required=True)
  pagesets_type = db.StringProperty()
  chromium_rev = db.StringProperty()
  skia_rev = db.StringProperty()
  completed_time = db.DateTimeProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'AdminTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'task_name': self.task_name,
            'pagesets_type': self.pagesets_type,
            'chromium_rev': self.chromium_rev,
            'skia_rev': self.skia_rev,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_all_admin_tasks(cls, offset):
    return (cls.all()
               .order('-requested_time')
               .fetch(offset=offset, limit=PAGINATION_LIMIT))

  @classmethod
  def get_all_admin_tasks_of_user(cls, user):
    return (cls.all()
               .filter('username =', user)
               .order('-requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_admin_task(cls, key):
    return db.GqlQuery(
        'SELECT * FROM AdminTasks WHERE __key__ = Key(\'AdminTasks\', %s);' % (
            key))

  @classmethod
  def delete_admin_task(cls, key):
    admin_tasks = cls.get_admin_task(key)
    if admin_tasks.count():
      admin_tasks[0].delete()


class LuaTasks(BaseTelemetryModel):
  """Data model for Lua tasks."""
  username = db.StringProperty(required=True)
  lua_script = db.TextProperty(required=True)
  lua_aggregator = db.TextProperty()
  lua_script_link = db.LinkProperty()
  lua_aggregator_link = db.LinkProperty()
  pagesets_type = db.StringProperty()
  chromium_rev = db.StringProperty()
  skia_rev = db.StringProperty()
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  lua_output_link = db.LinkProperty()
  description = db.StringProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'LuaTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'lua_script': self.lua_script,
            'lua_aggregator': self.lua_aggregator,
            'pagesets_type': self.pagesets_type,
            'chromium_rev': self.chromium_rev,
            'skia_rev': self.skia_rev,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_all_lua_tasks(cls, offset):
    return (cls.all()
               .order('-requested_time')
               .fetch(offset=offset, limit=PAGINATION_LIMIT))

  @classmethod
  def get_all_lua_tasks_of_user(cls, user):
    return (cls.all()
               .filter('username =', user)
               .order('-requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_lua_task(cls, key):
    return db.GqlQuery(
        'SELECT * FROM LuaTasks WHERE __key__ = Key(\'LuaTasks\', %s);' %  key)

  @classmethod
  def delete_lua_task(cls, key):
    lua_tasks = cls.get_lua_task(key)
    if lua_tasks.count():
      lua_tasks[0].delete()


class SkiaTryTasks(BaseTelemetryModel):
  """Data model for Skia Try tasks."""
  username = db.StringProperty(required=True)
  patch = db.BlobProperty()
  pagesets_type = db.StringProperty(required=True)
  chromium_rev = db.StringProperty(required=True)
  skia_rev = db.StringProperty(required=True)
  render_pictures_args = db.StringProperty(required=True)
  gpu_nopatch_run = db.BooleanProperty(default=False)
  gpu_withpatch_run = db.BooleanProperty(default=False)
  description = db.StringProperty()
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  patch_link = db.LinkProperty()
  slave1_output_link = db.LinkProperty()
  html_output_link = db.LinkProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'SkiaTryTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'patch': self.patch,
            'pagesets_type': self.pagesets_type,
            'chromium_rev': self.chromium_rev,
            'skia_rev': self.skia_rev,
            'render_pictures_args': self.render_pictures_args,
            'gpu_nopatch_run': self.gpu_nopatch_run,
            'gpu_withpatch_run': self.gpu_withpatch_run,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_all_skia_try_tasks(cls, offset):
    return (cls.all()
               .order('-requested_time')
               .fetch(offset=offset, limit=PAGINATION_LIMIT))

  @classmethod
  def get_all_skia_try_tasks_of_user(cls, user):
    return (cls.all()
               .filter('username =', user)
               .order('-requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_skia_try_task(cls, key):
    return db.GqlQuery(
        'SELECT * FROM SkiaTryTasks WHERE __key__ = '
        'Key(\'SkiaTryTasks\', %s);' % key)

  @classmethod
  def delete_skia_try_task(cls, key):
    skia_try_tasks = cls.get_skia_try_task(key)
    if skia_try_tasks.count():
      skia_try_tasks[0].delete()


class ChromiumTryTasks(BaseTelemetryModel):
  """Data model for Chromium Try tasks."""
  username = db.StringProperty(required=True)
  benchmark_name = db.StringProperty(required=True)
  benchmark_arguments = db.StringProperty()
  target_platform = db.StringProperty()
  pageset_type = db.StringProperty()
  skia_patch = db.BlobProperty()
  chromium_patch = db.BlobProperty()
  blink_patch = db.BlobProperty()
  num_repeated_runs = db.IntegerProperty()
  variance_threshold = db.FloatProperty(required=True)
  discard_outliers = db.FloatProperty(required=True)
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  description = db.StringProperty()
  skia_patch_link = db.LinkProperty()
  chromium_patch_link = db.LinkProperty()
  blink_patch_link = db.LinkProperty()
  build_log_link = db.LinkProperty()
  telemetry_nopatch_log_link = db.LinkProperty()
  telemetry_withpatch_log_link = db.LinkProperty()
  html_output_link = db.LinkProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'ChromiumTryTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'benchmark_name': self.benchmark_name,
            'benchmark_arguments': self.benchmark_arguments,
            'target_platform': self.target_platform,
            'pageset_type': self.pageset_type,
            'skia_patch': self.skia_patch,
            'chromium_patch': self.chromium_patch,
            'blink_patch': self.blink_patch,
            'num_repeated_runs': self.num_repeated_runs,
            'variance_threshold': self.variance_threshold,
            'discard_outliers': self.discard_outliers,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_all_chromium_try_tasks(cls, offset):
    return (cls.all()
               .order('-requested_time')
               .fetch(offset=offset, limit=PAGINATION_LIMIT))

  @classmethod
  def get_all_chromium_try_tasks_of_user(cls, user):
    return (cls.all()
               .filter('username =', user)
               .order('-requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_chromium_try_task(cls, key):
    return db.GqlQuery(
        'SELECT * FROM ChromiumTryTasks WHERE __key__ = '
        'Key(\'ChromiumTryTasks\', %s);' % key)

  @classmethod
  def delete_chromium_try_task(cls, key):
    chromium_try_tasks = cls.get_chromium_try_task(key)
    if chromium_try_tasks.count():
      chromium_try_tasks[0].delete()


class TelemetryTasks(BaseTelemetryModel):
  """Data model for Telemetry tasks."""
  username = db.StringProperty(required=True)
  benchmark_name = db.StringProperty(required=True)
  benchmark_arguments = db.StringProperty()
  pagesets_type = db.StringProperty()
  chromium_rev = db.StringProperty()
  skia_rev = db.StringProperty()
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  output_link = db.LinkProperty()
  whitelist_file = db.BlobProperty()
  description = db.StringProperty()

  def get_json_repr(self):
    """Returns a JSON representation of this Data Model."""
    return {
        'TelemetryTask': {
            'key': self.key().id_or_name(),
            'username': self.username,
            'chromium_rev': self.chromium_rev,
            'skia_rev': self.skia_rev,
            'benchmark_name': self.benchmark_name,
            'benchmark_arguments': self.benchmark_arguments,
            'pagesets_type': self.pagesets_type,
            'whitelist_file': self.whitelist_file,
            'requested_time': str(self.requested_time)
        }
    }

  @classmethod
  def get_completed_skp_runs(cls):
    """Returns all completed SKP runs."""
    return db.GqlQuery(
        'SELECT * FROM TelemetryTasks WHERE benchmark_name = '
        '\'skpicture_printer\' AND completed_time != null;')

  @classmethod
  def get_all_telemetry_tasks(cls, offset):
    return (cls.all()
               .order('-requested_time')
               .fetch(offset=offset, limit=PAGINATION_LIMIT))

  @classmethod
  def get_all_telemetry_tasks_of_user(cls, user):
    return (cls.all()
               .filter('username =', user)
               .order('-requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_telemetry_task(cls, key):
    return db.GqlQuery(
        'SELECT * FROM TelemetryTasks WHERE __key__ = '
        'Key(\'TelemetryTasks\', %s);' % key)

  @classmethod
  def delete_telemetry_task(cls, key):
    telemetry_tasks = cls.get_telemetry_task(key)
    if telemetry_tasks.count():
      telemetry_tasks[0].delete()


# List of Telemetry Data Models.
TELEMETRY_DATA_MODELS = (
    TelemetryTasks,
    AdminTasks,
    ChromiumBuilds,
    LuaTasks,
    ChromiumTryTasks,
    SkiaTryTasks
)


def add_telemetry_info_to_template(template_values, user_email,
                                   is_google_chromium_user):
  """Reads TelemetryInfo from the Datastore and adds it to the template."""
  telemetry_info = TelemetryInfo.get_telemetry_info()
  template_values['chrome_last_built'] = telemetry_info.chrome_last_built
  template_values['chromium_rev'] = telemetry_info.chromium_rev
  template_values['skia_rev'] = telemetry_info.skia_rev
  template_values['gce_slaves'] = telemetry_info.gce_slaves
  template_values['num_webpages'] = telemetry_info.num_webpages
  template_values['num_webpages_per_pageset'] = (
      telemetry_info.num_webpages_per_pageset)
  template_values['num_skp_files'] = telemetry_info.num_skp_files
  template_values['last_updated'] = telemetry_info.last_updated
  template_values['admin'] = user_email in TELEMETRY_ADMINS
  template_values['pdf_admin'] = user_email in PDF_ADMINS
  template_values['is_google_chromium_user'] = is_google_chromium_user
  template_values['pagesets_source'] = telemetry_info.pagesets_source
  template_values['framework_msg'] = telemetry_info.framework_msg


def get_skp_pagesets_to_builds():
  """Returns a map of pagesets to the builds that have SKPs."""
  completed_skp_runs = TelemetryTasks.get_completed_skp_runs()
  pagesets_to_builds = {}
  for completed_skp_run in completed_skp_runs:
    pagesets_type = completed_skp_run.pagesets_type
    chromium_rev = completed_skp_run.chromium_rev
    skia_rev = completed_skp_run.skia_rev
    if pagesets_type and chromium_rev and skia_rev:
      chromium_rev_date = ChromiumBuilds.get_chromium_build_with_revs(
          chromium_rev, skia_rev)[0].chromium_rev_date
      builds = pagesets_to_builds.get(pagesets_type, [])
      builds.append((chromium_rev, skia_rev, chromium_rev_date))
      builds.sort(cmp=lambda x, y: cmp(x[2], y[2]), reverse=True)
      pagesets_to_builds[pagesets_type] = builds
  return pagesets_to_builds


class AdminTasksPage(BasePage):
  """Displays the admin tasks page."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete admin task request.
    delete_key = self.request.get('delete')
    if delete_key:
      AdminTasks.delete_admin_task(delete_key)
      self.redirect('admin_tasks')
      return

    # It is an add admin task request.
    requested_time = datetime.datetime.now()
    admin_task = self.request.get('admin_task')
    pagesets_type = self.request.get('pagesets_type')
    if admin_task == 'Recreate Webpage Archives':
      chromium_rev, skia_rev = self.request.get('chromium_build').split('-')
    else:
      # Other admin tasks do not care which Chromium build is used.
      chromium_rev, skia_rev = (None, None)

    AdminTasks(
        username=self.user.email(),
        task_name=admin_task,
        pagesets_type=pagesets_type,
        chromium_rev=chromium_rev,
        skia_rev=skia_rev,
        requested_time=requested_time).put()
    self.redirect('admin_tasks')

  def _handle(self):
    """Sets the information to be displayed on the main page."""
    template_values = self.InitializeTemplate('Run Admin Tasks')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    admin_tasks = AdminTasks.get_all_admin_tasks_of_user(self.user.email())
    template_values['admin_tasks'] = admin_tasks
    template_values['pageset_types'] = PAGESET_TYPES
    chromium_builds = ChromiumBuilds.get_all_completed_chromium_builds()
    template_values['chromium_builds'] = chromium_builds
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()

    self.DisplayTemplate('admin_tasks.html', template_values)


class LuaScriptPage(BasePage):
  """Displays the lua script page."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete lua task request.
    delete_key = self.request.get('delete')
    if delete_key:
      LuaTasks.delete_lua_task(delete_key)
      self.redirect('lua_script')
      return

    # It is an add lua task request.
    requested_time = datetime.datetime.now()
    lua_script = db.Text(self.request.get('lua_script'))
    lua_aggregator = db.Text(self.request.get('lua_aggregator'))
    description = self.request.get('description')
    pagesets_type, chromium_rev, skia_rev = self.request.get(
        'pagesets_type_and_chromium_build').split('-')
    if not description:
      description = 'None'

    LuaTasks(
        username=self.user.email(),
        lua_script=lua_script,
        lua_aggregator=lua_aggregator,
        pagesets_type=pagesets_type,
        chromium_rev=chromium_rev,
        skia_rev=skia_rev,
        requested_time=requested_time,
        description=description).put()
    self.redirect('lua_script')

  def _handle(self):
    """Sets the information to be displayed on the main page."""
    template_values = self.InitializeTemplate(
        'Run Lua scripts on the SKP repository')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    lua_tasks = LuaTasks.get_all_lua_tasks_of_user(self.user.email())
    template_values['lua_tasks'] = lua_tasks
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()
    template_values['pagesets_to_builds'] = get_skp_pagesets_to_builds()

    self.DisplayTemplate('lua_script.html', template_values)


class ChromiumBuildsPage(BasePage):
  """Allows users to add and delete new chromium builds to the framework."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete chromium build request.
    delete_key = self.request.get('delete')
    if delete_key:
      ChromiumBuilds.delete_chromium_build(delete_key)
      self.redirect('chromium_builds')
      return

    # It is an add chromium build request.
    chromium_rev = self.request.get('chromium_rev')
    skia_rev = self.request.get('skia_rev')
    # If either is lkgr then get the commit hash from the lkgr urls.
    if chromium_rev == 'LKGR':
      chromium_rev = urllib2.urlopen(CHROMIUM_LKGR_URL).read()
    if skia_rev == 'LKGR':
      skia_rev = urllib2.urlopen(SKIA_LKGR_URL).read()

    # Only add a new build if it is not already in the repository.
    if (ChromiumBuilds.get_chromium_build_with_revs(
        chromium_rev, skia_rev).count() == 0):
      ChromiumBuilds(
          requested_time=datetime.datetime.now(),
          username=self.user.email(),
          chromium_rev=chromium_rev,
          skia_rev=skia_rev).put()
    self.redirect('chromium_builds')

  def _handle(self):
    """Sets template values to display."""
    template_values = self.InitializeTemplate('Chromium Builds')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    chromium_builds = ChromiumBuilds.get_all_chromium_builds()
    template_values['chromium_builds'] = chromium_builds
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()

    self.DisplayTemplate('chromium_builds.html', template_values)


class PendingTasksPage(BasePage):
  """Displays all pending tasks in the Cluster Telemetry queue."""

  @utils.require_user
  def get(self):
    template_values = self.InitializeTemplate('Pending Tasks')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)
    template_values['pending_tasks'] = get_all_pending_tasks()

    self.DisplayTemplate('tasks_queue.html', template_values)


class TelemetryInfoPage(BasePage):
  """Displays information messages."""

  @utils.require_user
  def post(self):
    self._handle()

  @utils.require_user
  def get(self):
    self._handle()

  def _handle(self):
    template_values = self.InitializeTemplate('Telemetry Info Message')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    info_msg = self.request.get('info_msg')
    template_values['info_msg'] = info_msg

    whitelist_key = self.request.get('whitelist_key')
    if whitelist_key:
      telemetry_task = TelemetryTasks.get_telemetry_task(whitelist_key)[0]
      template_values['whitelist_entries'] = (
          telemetry_task.whitelist_file.split())

    self.DisplayTemplate('skia_telemetry_info_page.html', template_values)


class AllTasks(BasePage):
  """Displays all tasks (Admin, Lua, Telemetry)."""

  @utils.require_user
  def get(self):
    return self._handle()

  def _handle(self):
    """Sets the information to be displayed on the main page."""
    template_values = self.InitializeTemplate('All Tasks')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)
    template_values['pagination_limit'] = PAGINATION_LIMIT
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()
    # The table shown on the all tasks page is the same as the other sub pages
    # except that the username is also shown and the delete button is not
    # shown.
    template_values['alltaskspage'] = True

    # Set template values for Admin, Lua and Telemetry datamodels.
    self.set_pagination_templates_for_models(
        template_values,
        'admin_tasks',
        AdminTasks.get_all_admin_tasks,
        tasks_counter(AdminTasks))
    self.set_pagination_templates_for_models(
        template_values,
        'lua_tasks',
        LuaTasks.get_all_lua_tasks,
        tasks_counter(LuaTasks))
    self.set_pagination_templates_for_models(
        template_values,
        'telemetry_tasks',
        TelemetryTasks.get_all_telemetry_tasks,
        tasks_counter(TelemetryTasks))
    self.set_pagination_templates_for_models(
        template_values,
        'chromium_try_tasks',
        ChromiumTryTasks.get_all_chromium_try_tasks,
        tasks_counter(ChromiumTryTasks))
    self.set_pagination_templates_for_models(
        template_values,
        'skia_try_tasks',
        SkiaTryTasks.get_all_skia_try_tasks,
        tasks_counter(SkiaTryTasks))

    self.DisplayTemplate('all_tasks.html', template_values)

  def set_pagination_templates_for_models(
      self, template_values, model_str, all_tasks_func, total_count):
    offset = int(self.request.get('%s_offset' % model_str, 0))
    all_tasks = all_tasks_func(offset=offset)
    template_values[model_str] = all_tasks
    if total_count > offset + PAGINATION_LIMIT:
      template_values['%s_next_offset' % model_str] = offset + PAGINATION_LIMIT
    if offset != 0:
      template_values['%s_prev_offset' % model_str] = offset - PAGINATION_LIMIT


class SkiaTryPage(BasePage):
  """Displays the Skia try page."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete chromium try task request.
    delete_key = self.request.get('delete')
    if delete_key:
      SkiaTryTasks.delete_skia_try_task(delete_key)
      self.redirect('skia_try')
      return

    # It is an add skia try task request.
    patch = db.Blob(str(self.request.get('patch')))
    pagesets_type, chromium_rev, skia_rev = self.request.get(
        'pagesets_type_and_chromium_build').split('-')
    render_pictures_args = self.request.get('render_pictures_args')
    gpu_nopatch_run = self.request.get('gpu_nopatch_run') == 'True'
    gpu_withpatch_run = self.request.get('gpu_withpatch_run') == 'True'
    description = self.request.get('description')
    if not description:
      description = 'None'
    requested_time = datetime.datetime.now()

    SkiaTryTasks(
        username=self.user.email(),
        patch=patch,
        pagesets_type=pagesets_type,
        chromium_rev=chromium_rev,
        skia_rev=skia_rev,
        render_pictures_args=render_pictures_args,
        gpu_nopatch_run=gpu_nopatch_run,
        gpu_withpatch_run=gpu_withpatch_run,
        requested_time=requested_time,
        description=description).put()
    self.redirect('skia_try')

  def _handle(self):
    """Sets the information to be displayed on the main page."""

    template_values = self.InitializeTemplate(
        'Cluster Telemetry Skia Tryserver')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    skia_try_tasks = SkiaTryTasks.get_all_skia_try_tasks_of_user(
        self.user.email())
    pagesets_to_builds = get_skp_pagesets_to_builds()
    # Only support all 10k pagesets for now.
    for pageset in pagesets_to_builds.keys():
      if '10k' not in pageset:
        del pagesets_to_builds[pageset]
    template_values['pagesets_to_builds'] = pagesets_to_builds
    template_values['skia_try_tasks'] = skia_try_tasks
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()
    template_values['pending_tasks_count'] = len(get_all_pending_tasks())

    self.DisplayTemplate('skia_try.html', template_values)


class ChromiumTryPage(BasePage):
  """Displays the Chromium try page."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete chromium try task request.
    delete_key = self.request.get('delete')
    if delete_key:
      ChromiumTryTasks.delete_chromium_try_task(delete_key)
      self.redirect('chromium_try')
      return

    # It is an add chromium try task request.
    benchmark_name = self.request.get('benchmark_name')
    benchmark_arguments = self.request.get('benchmark_arguments')
    target_platform = self.request.get('target_platform')
    pageset_type = self.request.get('pageset_type')
    num_repeated_runs = int(self.request.get('num_repeated_runs'))
    variance_threshold = float(self.request.get('variance_threshold'))
    discard_outliers = float(self.request.get('discard_outliers'))
    description = self.request.get('description')
    if not description:
      description = 'None'
    skia_patch = db.Blob(str(self.request.get('skia_patch')))
    chromium_patch = db.Blob(str(self.request.get('chromium_patch')))
    blink_patch = db.Blob(str(self.request.get('blink_patch')))
    requested_time = datetime.datetime.now()

    ChromiumTryTasks(
        username=self.user.email(),
        benchmark_name=benchmark_name,
        benchmark_arguments=benchmark_arguments,
        target_platform=target_platform,
        pageset_type=pageset_type,
        skia_patch=skia_patch,
        chromium_patch=chromium_patch,
        blink_patch=blink_patch,
        num_repeated_runs=num_repeated_runs,
        variance_threshold=variance_threshold,
        discard_outliers=discard_outliers,
        requested_time=requested_time,
        description=description).put()
    self.redirect('chromium_try')

  def _handle(self):
    """Sets the information to be displayed on the main page."""

    template_values = self.InitializeTemplate(
        'Cluster Telemetry Chromium Tryserver')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    chromium_try_tasks = ChromiumTryTasks.get_all_chromium_try_tasks_of_user(
        self.user.email())
    # Only support all 10k pagesets for now.
    template_values['pagesets'] = dict((k, v) for k, v in PAGESET_TYPES.items()
                                              if '10k' in k)
    template_values['supported_benchmarks'] = CHROMIUM_TRY_SUPPORTED_BENCHMARKS
    template_values['chromium_try_tasks'] = chromium_try_tasks
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()
    template_values['pending_tasks_count'] = len(get_all_pending_tasks())

    self.DisplayTemplate('chromium_try.html', template_values)


class LandingPage(BasePage):
  """Displays the main landing page of Skia Telemetry."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  def post(self):
    # Check if this is a delete telemetry task request.
    delete_key = self.request.get('delete')
    if delete_key:
      TelemetryTasks.delete_telemetry_task(delete_key)
      self.redirect('/skia-telemetry')
      return

    # It is an add telemetry task request.
    benchmark_name = self.request.get('benchmark_name')
    benchmark_arguments = self.request.get('benchmark_arguments')
    pagesets_type = self.request.get('pagesets_type')
    chromium_rev, skia_rev = self.request.get('chromium_build').split('-')
    requested_time = datetime.datetime.now()
    whitelist_file = self.request.get('whitelist_file')
    if whitelist_file:
      whitelist_file = db.Blob(whitelist_file)
    else:
      whitelist_file = None
    description = self.request.get('description')
    if not description:
      description = 'None'

    TelemetryTasks(
        username=self.user.email(),
        benchmark_name=benchmark_name,
        benchmark_arguments=benchmark_arguments,
        pagesets_type=pagesets_type,
        chromium_rev=chromium_rev,
        skia_rev=skia_rev,
        requested_time=requested_time,
        whitelist_file=whitelist_file,
        description=description).put()
    self.redirect('/skia-telemetry')


  def _handle(self):
    """Sets the information to be displayed on the main page."""

    template_values = self.InitializeTemplate('Skia Cluster Telemetry')

    add_telemetry_info_to_template(template_values, self.user.email(),
                                   self.is_admin)

    telemetry_tasks = TelemetryTasks.get_all_telemetry_tasks_of_user(
        self.user.email())
    template_values['telemetry_tasks'] = telemetry_tasks
    template_values['pageset_types'] = PAGESET_TYPES
    chromium_builds = ChromiumBuilds.get_all_completed_chromium_builds()
    template_values['chromium_builds'] = chromium_builds
    template_values['oldest_pending_task_key'] = get_oldest_pending_task_key()

    self.DisplayTemplate('skia_telemetry_landingpage.html', template_values)


class UpdateAdminTasksPage(BasePage):
  """Updates an admin task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    completed_time = datetime.datetime.now()
    admin_task = AdminTasks.get_admin_task(key)[0]
    admin_task.completed_time = completed_time
    admin_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class UpdateChromiumTryTasksPage(BasePage):
  """Updates a chromium try task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    skia_patch_link = self.request.get('skia_patch_link')
    chromium_patch_link = self.request.get('chromium_patch_link')
    blink_patch_link = self.request.get('blink_patch_link')
    build_log_link = self.request.get('build_log_link')
    telemetry_nopatch_log_link = self.request.get(
        'telemetry_nopatch_log_link')
    telemetry_withpatch_log_link = self.request.get(
        'telemetry_withpatch_log_link')
    html_output_link = self.request.get('html_output_link')
    completed_time = datetime.datetime.now()

    chromium_try_task = ChromiumTryTasks.get_chromium_try_task(key)[0]
    chromium_try_task.completed_time = completed_time
    chromium_try_task.skia_patch_link = skia_patch_link
    chromium_try_task.chromium_patch_link = chromium_patch_link
    chromium_try_task.blink_patch_link = blink_patch_link
    chromium_try_task.build_log_link = build_log_link
    chromium_try_task.telemetry_nopatch_log_link = telemetry_nopatch_log_link
    chromium_try_task.telemetry_withpatch_log_link = (
        telemetry_withpatch_log_link)
    chromium_try_task.html_output_link = html_output_link
    chromium_try_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('skia_patch_link: %s<br/>' % skia_patch_link)
    self.response.out.write('chromium_patch_link: %s<br/>' %
                            chromium_patch_link)
    self.response.out.write('blink_patch_link: %s<br/>' % blink_patch_link)
    self.response.out.write('build_log_link: %s<br/>' % build_log_link)
    self.response.out.write('telemetry_nopatch_log_link: %s<br/>' %
                                telemetry_nopatch_log_link)
    self.response.out.write('telemetry_withpatch_log_link: %s<br/>' %
                                telemetry_withpatch_log_link)
    self.response.out.write('html_output_link: %s<br/>' % html_output_link)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class UpdateSkiaTryTasksPage(BasePage):
  """Updates a chromium try task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    patch_link = self.request.get('patch_link')
    slave1_output_link = self.request.get('slave1_output_link')
    html_output_link = self.request.get('html_output_link')
    completed_time = datetime.datetime.now()

    skia_try_task = SkiaTryTasks.get_skia_try_task(key)[0]
    skia_try_task.completed_time = completed_time
    skia_try_task.patch_link = patch_link
    skia_try_task.slave1_output_link = slave1_output_link
    skia_try_task.html_output_link = html_output_link
    skia_try_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('patch_link: %s<br/>' % patch_link)
    self.response.out.write('slave1_output_link: %s<br/>' % slave1_output_link)
    self.response.out.write('html_output_link: %s<br/>' % html_output_link)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class UpdateChromiumBuildTasksPage(BasePage):
  """Updates a chromium build task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    build_log_link = self.request.get('build_log_link')
    chromium_rev_date = int(self.request.get('chromium_rev_date'))
    completed_time = datetime.datetime.now()

    chromium_build_task = ChromiumBuilds.get_chromium_build_with_key(key)[0]
    chromium_build_task.completed_time = completed_time
    chromium_build_task.build_log_link = build_log_link
    chromium_build_task.chromium_rev_date = datetime.datetime.fromtimestamp(
        chromium_rev_date)
    chromium_build_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('build_log_link: %s<br/>' % build_log_link)
    self.response.out.write('chromium_rev_date: %s<br/>' % chromium_rev_date)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class UpdateTelemetryTasksPage(BasePage):
  """Updates a telemetry task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    output_link = self.request.get('output_link')
    completed_time = datetime.datetime.now()

    telemetry_task = TelemetryTasks.get_telemetry_task(key)[0]
    telemetry_task.output_link = output_link
    telemetry_task.completed_time = completed_time
    telemetry_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('output_link: %s<br/>' % output_link)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class UpdateLuaTasksPage(BasePage):
  """Updates a lua task using its key."""

  @utils.admin_only
  def post(self):
    key = int(self.request.get('key'))
    lua_script_link = self.request.get('lua_script_link')
    lua_output_link = self.request.get('lua_output_link')
    completed_time = datetime.datetime.now()

    lua_task = LuaTasks.get_lua_task(key)[0]
    lua_task.lua_script_link = db.Link(lua_script_link)
    if self.request.get('lua_aggregator_link'):
      lua_task.lua_aggregator_link = db.Link(self.request.get(
          'lua_aggregator_link'))
    lua_task.lua_output_link = db.Link(lua_output_link)
    lua_task.completed_time = completed_time
    lua_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('lua_script_link: %s<br/>' % lua_script_link)
    self.response.out.write('lua_output_link: %s<br/>' % lua_output_link)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


def get_oldest_task_json_dict():
  """Returns the oldest pending task in a JSON dict."""
  # A list holding the different pending tasks.
  tasks = []

  for cls in TELEMETRY_DATA_MODELS:
    cls.add_oldest_pending_task(tasks)

  task_dict = {}
  if tasks:
    oldest_task = reduce(lambda x, y: x if x.requested_time < y.requested_time
                         else y, tasks)
    task_dict = oldest_task.get_json_repr()
  return task_dict


def get_oldest_pending_task_key():
  """Returns the key of the oldest pending task or -1 if no pending tasks."""
  oldest_pending_task_key = -1
  task_dict = get_oldest_task_json_dict()
  if task_dict:
    oldest_pending_task_key = task_dict.values()[0]['key']
  return oldest_pending_task_key


def get_all_pending_tasks():
  """Returns all pending tasks."""
  # A list holding the different pending tasks.
  pending_tasks = []

  for cls in TELEMETRY_DATA_MODELS:
    cls.add_pending_tasks_in_json(pending_tasks)

  # Sort the list according to the requested_times (oldest first).
  pending_tasks.sort(cmp=lambda x, y: cmp(x.values()[0]['requested_time'],
                                          y.values()[0]['requested_time']))
  return pending_tasks


class GetClusterTelemetryTasksPage(BasePage):
  """Returns a JSON of the oldest task in the queue."""

  def get(self):
    task_dict = get_oldest_task_json_dict()
    self.response.out.write(json.dumps(task_dict, indent=4, sort_keys=True))


class UpdateInfoPage(BasePage):
  """Updates Telemetry info from the GCE master."""

  @utils.admin_only
  def post(self):
    chrome_last_built = datetime.datetime.fromtimestamp(
        float(self.request.get('chrome_last_built')))
    chromium_rev = self.request.get('chromium_rev')
    skia_rev = self.request.get('skia_rev')
    gce_slaves = int(self.request.get('gce_slaves'))
    num_webpages = int(self.request.get('num_webpages'))
    num_webpages_per_pageset = int(self.request.get('num_webpages_per_pageset'))
    num_skp_files = int(self.request.get('num_skp_files'))
    last_updated = datetime.datetime.now()

    telemetry_info = TelemetryInfo.get_telemetry_info()
    # Save the last framework_msg if any.
    framework_msg = telemetry_info.framework_msg
    # Delete the old entry.
    telemetry_info.delete()

    # Add the new updated one.
    TelemetryInfo(
        chrome_last_built=chrome_last_built,
        chromium_rev=chromium_rev,
        skia_rev=skia_rev,
        gce_slaves=gce_slaves,
        num_webpages=num_webpages,
        num_webpages_per_pageset=num_webpages_per_pageset,
        num_skp_files=num_skp_files,
        framework_msg=framework_msg,
        last_updated=last_updated).put()

    self.response.out.write('<br/><br/>Added to the datastore-<br/><br/>')
    self.response.out.write('chrome_last_built: %s<br/>' % chrome_last_built)
    self.response.out.write('chromium_rev: %s<br/>' % chromium_rev)
    self.response.out.write('skia_rev: %s<br/>' % skia_rev)
    self.response.out.write('gce_slaves: %s<br/>' % gce_slaves)
    self.response.out.write('num_webpages: %s<br/>' % num_webpages)
    self.response.out.write('num_webpages_per_pageset: %s<br/>' % (
        num_webpages_per_pageset))
    self.response.out.write('num_skp_files: %s<br/>' % num_skp_files)
    self.response.out.write('last_updated: %s' % last_updated)


def bootstrap():
  # Guarantee that at least one instance of the required tables exist.
  if db.GqlQuery('SELECT __key__ FROM TelemetryInfo').get() is None:
    TelemetryInfo(
        chrome_last_built=datetime.datetime(1970, 1, 1),
        skia_rev='0',
        chromium_rev='0',
        gce_slaves=0,
        num_webpages=0,
        num_webpages_per_pageset=0,
        num_skp_files=0,
        last_updated=datetime.datetime.now()).put()

  if db.GqlQuery('SELECT __key__ FROM TelemetryTasks').get() is None:
    TelemetryTasks(
        username='Admin',
        benchmark_name='Test benchmark',
        benchmark_arguments='--test_arg',
        pagesets_type='Test type',
        requested_time=datetime.datetime.now(),
        completed_time=datetime.datetime.now()).put()

  if db.GqlQuery('SELECT __key__ FROM LuaTasks').get() is None:
    LuaTasks(
        username='Admin',
        lua_script='Test Lua Script',
        pagesets_type='Test type',
        requested_time=datetime.datetime.now(),
        completed_time=datetime.datetime.now()).put()

  if db.GqlQuery('SELECT __key__ FROM AdminTasks').get() is None:
    AdminTasks(
        username='Admin',
        task_name='Initial Table Creation',
        pagesets_type='Test type',
        requested_time=datetime.datetime.now(),
        completed_time=datetime.datetime.now()).put()


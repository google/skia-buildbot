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

PAGESET_TYPES = (
    'All',
    'Filtered',
    '100k',
    '10k',
    'Deeplinks',
)

# LKGR urls.
CHROMIUM_LKGR_URL = 'http://chromium-status.appspot.com/git-lkgr'
SKIA_LKGR_URL = 'http://skia-tree-status.appspot.com/git-lkgr'


class ChromiumBuilds(db.Model):
  """Datamodel for Chromium builds."""
  chromium_rev = db.StringProperty(required=True)
  skia_rev = db.StringProperty(required=True)
  username = db.StringProperty(required=True)
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  build_log_link = db.LinkProperty()
  chromium_rev_date = db.DateTimeProperty()

  @classmethod
  def get_all_chromium_builds(cls):
    return (cls.all()
               .order('-chromium_rev_date')
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
  def get_oldest_pending_chromium_build(cls):
    return (cls.all()
               .filter('completed_time =', None)
               .order('requested_time')
               .fetch(limit=1))

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


class AdminTasks(db.Model):
  """Data model for Admin tasks."""
  username = db.StringProperty(required=True)
  task_name = db.StringProperty(required=True)
  requested_time = db.DateTimeProperty(required=True)
  pagesets_type = db.StringProperty()
  completed_time = db.DateTimeProperty()

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
  def get_all_pending_admin_tasks(cls):
    return (cls.all()
               .filter('completed_time =', None)
               .order('requested_time')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def is_admin_task_running(cls, task_name):
    admin_tasks = (cls.all()
                      .filter('completed_time =', None)
                      .filter('task_name =', task_name)
                      .fetch(limit=1))
    return admin_tasks != None and len(admin_tasks) != 0

  @classmethod
  def delete_admin_task(cls, key):
    admin_tasks = cls.get_admin_task(key)
    if admin_tasks.count():
      admin_tasks[0].delete()


class LuaTasks(db.Model):
  """Data model for Lua tasks."""
  username = db.StringProperty(required=True)
  lua_script = db.TextProperty(required=True)
  lua_script_link = db.LinkProperty()
  pagesets_type = db.StringProperty()
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  lua_output_link = db.LinkProperty()
  description = db.StringProperty()

  @classmethod
  def get_all_pending_lua_tasks(cls):
    return (cls.all()
               .filter('completed_time =', None)
               .order('requested_time')
               .fetch(limit=FETCH_LIMIT))

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


class TelemetryTasks(db.Model):
  """Data model for Telemetry tasks."""
  username = db.StringProperty(required=True)
  benchmark_name = db.StringProperty(required=True)
  benchmark_arguments = db.StringProperty()
  pagesets_type = db.StringProperty()
  requested_time = db.DateTimeProperty(required=True)
  completed_time = db.DateTimeProperty()
  output_link = db.LinkProperty()
  whitelist_file = db.BlobProperty()
  description = db.StringProperty()

  @classmethod
  def get_all_pending_telemetry_tasks(cls):
    return (cls.all()
               .filter('completed_time =', None)
               .order('requested_time')
               .fetch(limit=FETCH_LIMIT))

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
  def is_skp_benchmark_running(cls):
    skp_benchmarks = (cls.all()
                         .filter('completed_time =', None)
                         .filter('benchmark_name =', 'skpicture_printer')
                         .fetch(limit=1))
    return skp_benchmarks != None and len(skp_benchmarks) != 0

  @classmethod
  def delete_telemetry_task(cls, key):
    telemetry_tasks = cls.get_telemetry_task(key)
    if telemetry_tasks.count():
      telemetry_tasks[0].delete()


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

    # There should be only one instance of an admin task running at a time.
    # Running multiple instances causes unpredictable and inconsistent behavior.
    if AdminTasks.is_admin_task_running(admin_task):
      self.redirect('/skia-telemetry/skia_telemetry_info_page?info_msg=%s'
                    ' is already running!' % admin_task)
    else:
      AdminTasks(
          username=self.user.email(),
          task_name=admin_task,
          pagesets_type=pagesets_type,
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
    description = self.request.get('description')
    pagesets_type = self.request.get('pagesets_type')
    if not description:
      description = 'None'

    # Lua scripts should not run if skpicture_printer is already running,
    # because when that benchmark is running the local skps directory is empty
    # till the benchmark completes (this takes a few hours).
    if TelemetryTasks.is_skp_benchmark_running():
      self.redirect('/skia-telemetry/skia_telemetry_info_page?info_msg=SKP '
                    'files are in the process of being captured. Please try '
                    'your Lua script in a few hours.')
    else:
      LuaTasks(
          username=self.user.email(),
          lua_script=lua_script,
          pagesets_type=pagesets_type,
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
    template_values['pageset_types'] = PAGESET_TYPES

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

    self.DisplayTemplate('chromium_builds.html', template_values)


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
    requested_time = datetime.datetime.now()
    whitelist_file = self.request.get('whitelist_file')
    if whitelist_file:
      whitelist_file = db.Blob(whitelist_file)
    else:
      whitelist_file = None
    description = self.request.get('description')
    if not description:
      description = 'None'

    # There should be only one instance of a skp benchmark running at a time.
    # Running multiple instances causes unpredictable and inconsistent behavior.
    if (benchmark_name == 'skpicture_printer' and
        TelemetryTasks.is_skp_benchmark_running()):
      self.redirect('/skia-telemetry/skia_telemetry_info_page?info_msg=%s'
                    ' is already running!' % benchmark_name)
    else:
      TelemetryTasks(
          username=self.user.email(),
          benchmark_name=benchmark_name,
          benchmark_arguments=benchmark_arguments,
          pagesets_type=pagesets_type,
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

    self.DisplayTemplate('skia_telemetry_landingpage.html', template_values)


class GetAdminTasksPage(BasePage):
  """Returns a JSON of all pending admin tasks in the queue."""

  def get(self):
    admin_tasks = AdminTasks.get_all_pending_admin_tasks()
    # Create a dict for JSON from the admin pending tasks.
    tasks_dict = {}
    count = 1
    for task in admin_tasks:
      task_dict = {}
      task_dict['key'] = task.key().id_or_name()
      task_dict['username'] = task.username
      task_dict['task_name'] = task.task_name
      task_dict['pagesets_type'] = task.pagesets_type
      tasks_dict[count] = task_dict
      count += 1
    self.response.out.write(json.dumps(tasks_dict, sort_keys=True))


class GetChromiumBuildTasksPage(BasePage):
  """Returns a JSON of the oldest pending chromium build task in the queue."""

  def get(self):
    chromium_build_task = ChromiumBuilds.get_oldest_pending_chromium_build()

    # Create a dict for JSON from the oldest pending task.
    if chromium_build_task:
      task = chromium_build_task[0]
      task_dict = {
        1: {
            'key': task.key().id_or_name(),
            'username': task.username,
            'chromium_rev': task.chromium_rev,
            'skia_rev': task.skia_rev
        }
      }
    else:
      task_dict = {}
    self.response.out.write(json.dumps(task_dict, sort_keys=True))


class GetLuaTasksPage(BasePage):
  """Returns a JSON of all pending lua tasks in the queue."""

  def get(self):
    lua_tasks = LuaTasks.get_all_pending_lua_tasks()
    # Create a dict for JSON from the lua pending tasks.
    tasks_dict = {}
    count = 1
    for task in lua_tasks:
      task_dict = {}
      task_dict['key'] = task.key().id_or_name()
      task_dict['username'] = task.username
      task_dict['lua_script'] = task.lua_script
      task_dict['pagesets_type'] = task.pagesets_type
      tasks_dict[count] = task_dict
      count += 1
    self.response.out.write(json.dumps(tasks_dict, sort_keys=True))


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
    lua_task.lua_output_link = db.Link(lua_output_link)
    lua_task.completed_time = completed_time
    lua_task.put()

    self.response.out.write('<br/><br/>Updated the datastore-<br/><br/>')
    self.response.out.write('key: %s<br/>' % key)
    self.response.out.write('lua_script_link: %s<br/>' % lua_script_link)
    self.response.out.write('lua_output_link: %s<br/>' % lua_output_link)
    self.response.out.write('completed_time: %s<br/>' % completed_time)


class GetTelemetryTasksPage(BasePage):
  """Returns a JSON of all telemetry tasks in the queue."""

  def get(self):
    telemetry_tasks = TelemetryTasks.get_all_pending_telemetry_tasks()
    # Create a dict for JSON from the telemetry pending tasks.
    tasks_dict = {}
    count = 1
    for task in telemetry_tasks:
      task_dict = {}
      task_dict['key'] = task.key().id_or_name()
      task_dict['username'] = task.username
      task_dict['benchmark_name'] = task.benchmark_name
      task_dict['benchmark_arguments'] = task.benchmark_arguments
      task_dict['pagesets_type'] = task.pagesets_type
      task_dict['whitelist_file'] = task.whitelist_file
      tasks_dict[count] = task_dict
      count += 1
    self.response.out.write(json.dumps(tasks_dict, indent=4, sort_keys=True))


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


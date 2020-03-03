# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Status management pages."""

from contextlib import closing

import base64
import datetime
import json
import logging
import re
import urllib2

from google.appengine.api import memcache
from google.appengine.ext import db

from base_page import BasePage
from sheriff import SheriffSchedules
import utils


OPEN_STATE = 'open'
CAUTION_STATE = 'caution'
CLOSED_STATE = 'closed'

# The maximum chunk of statuses that are displayed.
MAX_STATUS_CHUNK = 1000
# The default chunk of statuses that are displayed.
DEFAULT_STATUS_CHUNK = 25

CHROMIUM_DEPS_FILE = (
    'https://chromium.googlesource.com/chromium/src/+/master/DEPS?format=TEXT')


class Status(db.Model):
  """Description for the status table."""
  # The username who added this status.
  username = db.StringProperty(required=True)
  # The date when the status got added.
  date = db.DateTimeProperty(auto_now_add=True)
  # The message. It can contain html code.
  message = db.StringProperty(required=True)

  @property
  def general_state(self):
    """Returns a string representing the state that the status message
    describes.
    """
    if re.search(CLOSED_STATE, self.message, re.IGNORECASE):
      return CLOSED_STATE
    elif re.search(CAUTION_STATE, self.message, re.IGNORECASE):
      return CAUTION_STATE
    else:
      return OPEN_STATE

  @staticmethod
  def validate_state_message(message):
    """Throws an Error iff exactly one of closed, open or caution is missing."""
    closed_state = re.search(CLOSED_STATE, message, re.IGNORECASE)
    caution_state = re.search(CAUTION_STATE, message, re.IGNORECASE)
    open_state = re.search(OPEN_STATE, message, re.IGNORECASE)
    if (closed_state and open_state) or (
        closed_state and caution_state) or (
        caution_state and open_state):
      raise ValueError(
          'Cannot specify two keywords from (\'%s\', \'%s\', \'%s\') in a '
          'status message!' % (CLOSED_STATE, CAUTION_STATE, OPEN_STATE))
    elif not (closed_state or caution_state or open_state):
      raise ValueError(
          'Must specify either \'%s\' or \'%s\' or \'%s\' somewhere in the '
          'status message!' % (CLOSED_STATE, CAUTION_STATE, OPEN_STATE))

  @property
  def can_commit_freely(self):
    return (self.general_state == OPEN_STATE or
            self.general_state == CAUTION_STATE)

  def AsDict(self):
    data = super(Status, self).AsDict()
    data['general_state'] = self.general_state
    data['can_commit_freely'] = self.can_commit_freely
    return data


def get_status():
  """Returns the current Status, e.g. the most recent one."""
  status = memcache.get('last_status')
  if status is None:
    status = Status.all().order('-date').get()
    # Use add instead of set(); must not change it if it was already set.
    memcache.add('last_status', status)
  return status


def put_status(status):
  """Sets the current Status, e.g. append a new one."""
  prev_status = memcache.get('last_status')
  if prev_status is None:
    prev_status = Status.all().order('-date').get()
  prev_status.put()
  # Now add the new status.
  status.put()

  # Flush the cache.
  memcache.flush_all()
  memcache.set('last_status', status)
  memcache.delete('last_statuses')


def get_last_statuses(limit):
  """Returns the last |limit| statuses."""
  statuses = memcache.get('last_statuses')
  if not statuses or len(statuses) < limit:
    statuses = Status.all().order('-date').fetch(limit)
    memcache.add('last_statuses', statuses)
  return statuses[:limit]


def parse_date(date):
  """Parses a date."""
  match = re.match(r'^(\d\d\d\d)-(\d\d)-(\d\d)$', date)
  if match:
    return datetime.datetime(
        int(match.group(1)), int(match.group(2)), int(match.group(3)))
  if date.isdigit():
    return datetime.datetime.utcfromtimestamp(int(date))
  return None


class AllStatusPage(BasePage):
  """Displays a big chunk, equal to MAX_CHUNK of status values."""
  def get(self):
    query = db.Query(Status).order('-date')
    start_date = self.request.get('startTime')
    if start_date:
      query.filter('date <', parse_date(start_date))
    try:
      limit = int(self.request.get('limit'))
    except ValueError:
      limit = MAX_STATUS_CHUNK
    end_date = self.request.get('endTime')
    beyond_end_of_range_status = None
    if end_date:
      query.filter('date >=', parse_date(end_date))
      # We also need to get the very next status in the range, otherwise
      # the caller can't tell what the effective tree status was at time
      # |end_date|.
      beyond_end_of_range_status = Status.all(
          ).filter('date <', end_date).order('-date').get()

    out_format = self.request.get('format', 'csv')
    if out_format == 'csv':
      # It's not really an html page.
      self.response.headers['Content-Type'] = 'text/plain'
      template_values = self.InitializeTemplate(self.APP_NAME + ' Tree Status')
      template_values['status'] = query.fetch(limit)
      template_values['beyond_end_of_range_status'] = beyond_end_of_range_status
      self.DisplayTemplate('allstatus.html', template_values)
    elif out_format == 'json':
      self.response.headers['Content-Type'] = 'application/json'
      self.response.headers['Access-Control-Allow-Origin'] = '*'
      statuses = [s.AsDict() for s in query.fetch(limit)]
      if beyond_end_of_range_status:
        statuses.append(beyond_end_of_range_status.AsDict())
      data = json.dumps(statuses)
      callback = self.request.get('callback')
      if callback:
        if re.match(r'^[a-zA-Z$_][a-zA-Z$0-9._]*$', callback):
          data = '%s(%s);' % (callback, data)
      self.response.out.write(data)
    else:
      self.response.headers['Content-Type'] = 'text/plain'
      self.response.out.write('Invalid format')


class LkgrPage(BasePage):
  """Displays Skia's LKGR.

  Parses Chromium's DEPS file to get this information. The justification for
  this is that a Skia rev makes it into Chromium after passing all required
  tests.
  """

  def get(self):
    """Displays Skia's LKGR."""
    try:
      with closing(urllib2.urlopen(CHROMIUM_DEPS_FILE)) as f:
        chromium_deps = base64.b64decode(f.read())
      if 'skia_revision' in chromium_deps:
        skia_lkgr = re.search(
            r'.*\'skia_revision\': \'(?P<revision>[0-9a-fA-F]{2,40})\'.*',
            chromium_deps).group('revision')
      else:
        raise Exception('Could not find skia_revision!')
    except Exception, e:
      skia_lkgr = -1
      logging.error(e)
    self.response.out.write(skia_lkgr)


class BannerStatusPage(BasePage):
  """Displays the /current page."""

  def get(self):
    """Displays the current message and nothing else."""
    out_format = self.request.get('format', 'html')
    status = get_status()
    if out_format == 'raw':
      self.response.headers['Content-Type'] = 'text/plain'
      self.response.out.write(status.message)
    elif out_format == 'json':
      self.response.headers['Content-Type'] = 'application/json'
      if self.request.get('with_credentials'):
        self.response.headers['Access-Control-Allow-Origin'] = (
            'gerrit-int.chromium.org, gerrit.chromium.org')
        self.response.headers['Access-Control-Allow-Credentials'] = 'true'
      else:
        self.response.headers['Access-Control-Allow-Origin'] = '*'
      data = json.dumps(status.AsDict())
      callback = self.request.get('callback')
      if callback:
        if re.match(r'^[a-zA-Z$_][a-zA-Z$0-9._]*$', callback):
          data = '%s(%s);' % (callback, data)
      self.response.out.write(data)
    elif out_format == 'html':
      template_values = self.InitializeTemplate(self.APP_NAME + ' Tree Status')
      template_values['message'] = status.message
      template_values['state'] = status.general_state
      template_values['sheriff'] = SheriffSchedules.get_current_sheriff()
      self.DisplayTemplate('current.html', template_values, use_cache=True)
    else:
      self.error(400)


class BinaryStatusPage(BasePage):
  """Displays the /binarystatus page."""

  def get(self):
    """Displays 1 if the tree is open or in caution, and 0 if it is closed."""
    status = get_status()
    self.response.headers['Cache-Control'] = 'no-cache, private, max-age=0'
    self.response.headers['Content-Type'] = 'text/plain'
    self.response.out.write(str(int(status.can_commit_freely)))

  @utils.admin_only
  def post(self):
    """Adds a new message from a backdoor.

    The main difference with MainPage.post() is that it doesn't look for
    conflicts and doesn't redirect to /.
    """
    message = self.request.get('message')
    username = self.request.get('username')
    if message and username:
      put_status(Status(message=message, username=username))
    self.response.out.write('OK')


class MainPage(BasePage):
  """Displays the main page containing the last DEFAULT_STATUS_CHUNK msgs."""

  @utils.require_user
  def get(self):
    self.redirect("https://tree-status.skia.org/")
    return

  def _handle(self, error_message='', last_message=''):
    """Sets the information to be displayed on the main page."""
    try:
      limit = min(max(int(self.request.get('limit')), 1), MAX_STATUS_CHUNK)
    except ValueError:
      limit = DEFAULT_STATUS_CHUNK
    status = get_last_statuses(limit)
    current_status = get_status()
    if not last_message:
      last_message = current_status.message

    template_values = self.InitializeTemplate(self.APP_NAME + ' Tree Status')
    template_values['status'] = status
    template_values['message'] = last_message
    template_values['last_status_key'] = current_status.key()
    template_values['error_message'] = error_message
    template_values['default_status_chunk'] = DEFAULT_STATUS_CHUNK
    self.DisplayTemplate('main.html', template_values)

  @utils.require_user
  @utils.admin_only
  def post(self):
    """Adds a new message."""
    # We pass these variables back into get(), prepare them.
    last_message = ''
    error_message = ''

    # Get the posted information.
    new_message = self.request.get('message')
    last_status_key = self.request.get('last_status_key')
    if not new_message:
      # A submission contained no data. It's a better experience to redirect
      # in this case.
      self.redirect("/")
      return

    # Ensure the new status message contains exactly one of 'open' or 'closed'
    # or 'caution'.
    try:
      Status.validate_state_message(new_message)
    except ValueError, e:
      error_message = e.message
      return self._handle(e.message, new_message)

    current_status = get_status()
    if current_status and (last_status_key != str(current_status.key())):
      error_message = ('Message not saved, mid-air collision detected, '
                       'please resolve any conflicts and try again!')
      last_message = new_message
      return self._handle(error_message, last_message)
    else:
      put_status(Status(message=new_message, username=self.user.email()))
      self.redirect("/")


def bootstrap():
  # Guarantee that at least one instance exists.
  if db.GqlQuery('SELECT __key__ FROM Status').get() is None:
    Status(username='none', message='welcome to status').put()

# Copyright (c) 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""AutoRoll Bot (ARB) management pages."""


import json
import time

from google.appengine.api import memcache
from google.appengine.ext import db

from base_page import BasePage
import utils


# How much history to display on the arb status page.
HISTORY_LIMIT = 25

# Memcache keys used by the ARB pages.
ARB_STATUS_MEMCACHE_KEY = 'arb_status'
IS_STOPPED_MEMCACHE_KEY = 'arb_is_stopped'


class ARBStatus(db.Model):
  """The status of the ARB as reported by the bot."""
  # The last status reported by the ARB.
  last_reported_status = db.StringProperty(required=True)
  # The codereview link of the DEPS roll.
  deps_roll_link = db.LinkProperty()
  # The revision that was used for the last DEPS roll.
  last_roll_rev = db.StringProperty()
  # The revision that is used for the current DEPS roll.
  curr_roll_rev = db.StringProperty()
  # The date when the status was modified.
  date = db.DateTimeProperty(auto_now=True)

  @staticmethod
  def get_arb_status():
    """Returns the last reported ARB status."""
    arb_status = memcache.get(ARB_STATUS_MEMCACHE_KEY)
    if arb_status == None:
      arb_status =  ARBStatus.all().get()
      memcache.add(ARB_STATUS_MEMCACHE_KEY, arb_status)
    return arb_status

  @staticmethod
  def flush_cache():
    """Flushes the memcache key used by this datamodel."""
    memcache.flush_all()
    memcache.delete(ARB_STATUS_MEMCACHE_KEY)
    # Wait for a second before accessing the memcache again (in case there is a
    # propagation delay).
    time.sleep(1)


class ARBStoppedAction(db.Model):
  """Contains information of whether the ARB was stopped/started."""
  # The user set status of the ARB.
  is_stopped = db.BooleanProperty(required=True)
  # The username that stopped/started the ARB.
  username = db.StringProperty(required=True)
  # The reason ARB was stopped/started.
  reason = db.StringProperty()
  # The date when this data model was added.
  date = db.DateTimeProperty(auto_now_add=True)

  @staticmethod
  def currently_stopped():
    """Returns the latest user specified setting."""
    is_stopped = memcache.get(IS_STOPPED_MEMCACHE_KEY)
    if is_stopped == None:
      is_stopped = ARBStoppedAction.all().order('-date').get().is_stopped
      memcache.add(IS_STOPPED_MEMCACHE_KEY, is_stopped)
    return is_stopped

  @staticmethod
  def flush_cache():
    """Flushes the memcache key used by this datamodel."""
    memcache.flush_all()
    memcache.delete(IS_STOPPED_MEMCACHE_KEY)
    # Wait for a second before accessing the memcache again (in case there is a
    # propagation delay).
    time.sleep(1)

  @staticmethod
  def get_actions_history(limit=HISTORY_LIMIT):
    return ARBStoppedAction.all().order('-date').fetch(HISTORY_LIMIT)


class GetARBStatusPage(BasePage):
  """Returns the current status of the ARB in JSON."""
  def get(self):
    arb_status = ARBStatus.get_arb_status()
    json_dict = {
        'status': arb_status.last_reported_status,
        'deps_roll_link': arb_status.deps_roll_link,
        'last_roll_rev': arb_status.last_roll_rev,
        'curr_roll_rev': arb_status.curr_roll_rev,
        'date': str(arb_status.date),
     }
    json_output = json.dumps(json_dict)
    self.response.headers['Content-Type'] = 'application/json'
    self.response.headers['Access-Control-Allow-Origin'] = '*'
    self.response.out.write(json_output)


class SetARBStatusPage(BasePage):
  """Allows the ARB to set its current status."""
  @utils.admin_only
  def post(self):
    status = self.request.get('status')
    deps_roll_link = self.request.get('deps_roll_link', None)
    last_roll_rev = self.request.get('last_roll_rev', None)
    curr_roll_rev = self.request.get('curr_roll_rev', None)
    arb_status = ARBStatus.get_arb_status()
    arb_status.last_reported_status = status
    arb_status.deps_roll_link = deps_roll_link
    arb_status.last_roll_rev = last_roll_rev
    arb_status.curr_roll_rev = curr_roll_rev
    arb_status.put()
    # Flush the cache.
    ARBStatus.flush_cache()


class ARBIsStoppedPage(BasePage):
  """Returns whether the ARB has been stopped/started."""
  def get(self):
    is_stopped = ARBStoppedAction.currently_stopped()
    json_dict = {'is_stopped': is_stopped}
    json_output = json.dumps(json_dict)
    self.response.headers['Content-Type'] = 'application/json'
    self.response.out.write(json_output)


class SetARBActionPage(BasePage):
  """Allows users to stop/start the ARB."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  @utils.admin_only
  def post(self):
    new_action = ARBStoppedAction(
        is_stopped=bool(self.request.get('is_stopped')),
        username=self.user.email(),
        reason=self.request.get('reason'))
    new_action.put()
    # Flush the cache.
    ARBStoppedAction.flush_cache()
    # Set template values and load the template.
    self._handle()

  def _handle(self):
    """Sets the information to be displayed on the builder status page."""
    template_values = self.InitializeTemplate('AutoRoll Bot Status')
    template_values['arb_status'] = ARBStatus.get_arb_status()
    template_values['user_actions'] = ARBStoppedAction.get_actions_history()
    template_values['history_limit'] = HISTORY_LIMIT
    self.DisplayTemplate('arb_status.html', template_values)


def bootstrap():
  # Guarantee that at least one instance exists.
  if db.GqlQuery('SELECT __key__ FROM ARBStatus').get() is None:
    ARBStatus(last_reported_status='In progress',
              deps_roll_link='https://codereview.chromium.org/923103002/',
              last_roll_rev='f16c00e41b72daa81ed7efacbead06b387767841',
              curr_roll_rev='f89f60f6972569a41fa737c786d238559027fede').put()
  if db.GqlQuery('SELECT __key__ FROM ARBStoppedAction').get() is None:
    ARBStoppedAction(is_stopped=False, username='rmistry@google.com',
                     reason='Initial entry').put()


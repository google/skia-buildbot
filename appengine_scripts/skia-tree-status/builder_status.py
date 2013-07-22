# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Builder Status management pages."""


import json
import time

from google.appengine.api import memcache
from google.appengine.ext import db

from base_page import BasePage
import utils


# The default number of builder status messages that are displayed.
DEFAULT_BUILDER_STATUS_CHUNK = 500


class BuilderStatus(db.Model):
  """Description for each builder."""
  # The username who added this status.
  username = db.StringProperty(required=True)
  # The builder the status message applies to.
  builder_name = db.StringProperty(required=True)
  # The date when the status got added.
  date = db.DateTimeProperty(auto_now_add=True)
  # The message. It can contain html code.
  message = db.StringProperty(required=True)

  @classmethod
  def delete_builder(cls, builder_name):
    builders = cls.all().filter('builder_name =', builder_name).fetch(limit=1)
    if builders:
      builders[0].delete()

  @classmethod
  def get_status_for_builder(cls, builder_name):
    builders = (BuilderStatus.all()
                             .filter('builder_name =', builder_name)
                             .fetch(limit=1))
    return builders[0].message if builders else ''


def get_builder_statuses():
  """Returns all builder statuses."""
  builder_statuses = memcache.get('builder_statuses')
  if not builder_statuses:
    builder_statuses = BuilderStatus.all().order('-date').fetch(
        DEFAULT_BUILDER_STATUS_CHUNK)
    memcache.add('builder_statuses', builder_statuses)
  return builder_statuses


class GetBuilderStatusesPage(BasePage):
  """Returns a JSON of all builder statuses."""

  def get(self):
    builder_statuses = get_builder_statuses()
    builder_dict = {}
    for status in builder_statuses:
      tmp_dict = {}
      tmp_dict['message'] = status.message
      tmp_dict['date'] = str(status.date)
      tmp_dict['username'] = status.username
      builder_dict[status.builder_name] = tmp_dict
    html_output = json.dumps(builder_dict, sort_keys=True)
    jsonp = self.request.get('jsonp')
    if jsonp:
      html_output = jsonp + '(' + html_output + ')'
    self.response.out.write(html_output)


class BuilderStatusPage(BasePage):
  """Displays the builder status page containing msgs for some builders."""

  @utils.require_user
  def get(self):
    return self._handle()

  @utils.require_user
  @utils.admin_only
  def post(self):
    """Adds a new builder status message."""

    # Check if this is a delete builder request.
    delete_builder = self.request.get('delete')
    if delete_builder:
      BuilderStatus.delete_builder(delete_builder)
    else:
      message = self.request.get('message')
      builder_name = self.request.get('builder_name')
      # If an entry with the builder_name already exists then delete it first.
      BuilderStatus.delete_builder(builder_name)

      # Add a new BuilderStatus entry.
      BuilderStatus(username=self.user.email(),
                    builder_name=builder_name,
                    message=message).put()

    # Flush the cache.
    memcache.flush_all()
    memcache.delete('builder_statuses')
    # Wait for a second before accessing the memcache again (in case there is a
    # propagation delay).
    time.sleep(1)
    # Set template values and load the template.
    self._handle()

  def _handle(self):
    """Sets the information to be displayed on the builder status page."""
    builder_statuses = get_builder_statuses()

    template_values = self.InitializeTemplate('Builder Statuses')
    template_values['builder_statuses'] = builder_statuses
    selected_builder_name = self.request.get('selected_builder_name')
    template_values['selected_builder_name'] = selected_builder_name
    if selected_builder_name:
      template_values['selected_builder_status'] = (
          BuilderStatus.get_status_for_builder(selected_builder_name))
    self.DisplayTemplate('builder_status.html', template_values)


def bootstrap():
  # Guarantee that at least one instance exists.
  if db.GqlQuery('SELECT __key__ FROM BuilderStatus').get() is None:
    BuilderStatus(username='none', builder_name='TestBuilder',
           message='welcome to status').put()


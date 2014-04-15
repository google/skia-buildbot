# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Sheriff schedule pages."""


import datetime
import json

from google.appengine.ext import db

from base_page import BasePage
import utils


FETCH_LIMIT = 50

HEADER_CONTENT_TYPE = 'Content-Type'
HEADER_ACCESS_CONTROL_ALLOW_ORIGIN = 'Access-Control-Allow-Origin'

JSON_CONTENT_TYPE = 'application/json'


class Sheriffs(db.Model):
  """Contains the list of Sheriffs"""
  username = db.StringProperty(required=True)

  @classmethod
  def get_all_sheriffs(cls):
    return cls.all().fetch(limit=FETCH_LIMIT)


class SheriffSchedules(db.Model):
  """A single sheriff oncall rotation (one person, one time interval)."""
  schedule_start = db.DateTimeProperty(required=True)
  schedule_end = db.DateTimeProperty(required=True)
  username = db.StringProperty(required=True)

  @classmethod
  def get_upcoming_schedules(cls):
    return (cls.all()
               .filter('schedule_end >', datetime.datetime.now())
               .order('schedule_end')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_schedules_for_sheriff(cls, sheriff):
    return (cls.all()
               .filter('username =', sheriff)
               .order('schedule_end')
               .fetch(limit=FETCH_LIMIT))

  @classmethod
  def get_current_sheriff(cls):
    current_schedule = cls.all().filter(
        'schedule_end >', datetime.datetime.now()).get()
    if current_schedule:
      return current_schedule.username
    else:
      return 'None'

  @classmethod
  def delete_schedules_after(cls, datetime_obj):
    schedules = cls.all().filter(
        'schedule_end >', datetime_obj).fetch(limit=FETCH_LIMIT)
    for schedule in schedules:
      schedule.delete()

  def AsDict(self, display_year=False):
    data = super(SheriffSchedules, self).AsDict()
    data['username'] = self.username

    date_format = '%m/%d/%y' if display_year else '%m/%d'
    data['schedule_start'] = self.schedule_start.strftime(date_format)
    data['schedule_end'] = self.schedule_end.strftime(date_format)
    return data


class QuerySheriffPage(BasePage):
  """Displays the schedules of the provided sheriff."""

  def get(self):
    self.response.headers[HEADER_CONTENT_TYPE] = JSON_CONTENT_TYPE
    self.response.headers[HEADER_ACCESS_CONTROL_ALLOW_ORIGIN] = '*'

    username = self.request.get('username')
    data = json.dumps({})
    for sheriff in Sheriffs.get_all_sheriffs():
      if sheriff.username.startswith(username):
        schedules = []
        for schedule in SheriffSchedules.get_schedules_for_sheriff(
            sheriff.username):
          schedules.append(schedule.AsDict(display_year=True))
        data = json.dumps({sheriff.username: schedules})
        break

    self.response.out.write(data)


class CurrentSheriffPage(BasePage):
  """Displays the current sheriff and schedule in JSON."""

  def get(self):
    self.response.headers[HEADER_CONTENT_TYPE] = JSON_CONTENT_TYPE
    self.response.headers[HEADER_ACCESS_CONTROL_ALLOW_ORIGIN] = '*'

    upcoming_schedules = SheriffSchedules.get_upcoming_schedules()
    if upcoming_schedules:
      data = json.dumps(upcoming_schedules[0].AsDict())
    else:
      data = json.dumps({})
    callback = self.request.get('callback')
    if callback:
      data = callback + '(' + data + ')'
    self.response.out.write(data)


class NextSheriffPage(BasePage):
  """Displays the next sheriff and schedule in JSON."""

  def get(self):
    self.response.headers[HEADER_CONTENT_TYPE] = JSON_CONTENT_TYPE
    self.response.headers[HEADER_ACCESS_CONTROL_ALLOW_ORIGIN] = '*'

    upcoming_schedules = SheriffSchedules.get_upcoming_schedules()
    if upcoming_schedules and len(upcoming_schedules) > 1:
      data = json.dumps(upcoming_schedules[1].AsDict())
    else:
      data = json.dumps({})
    callback = self.request.get('callback')
    if callback:
      data = callback + '(' + data + ')'
    self.response.out.write(data)


class SheriffPage(BasePage):
  """Displays the list and rotation schedule of all sheriffs."""

  @utils.require_user
  def get(self):
    return self._handle()

  def _handle(self, error_message='', last_message=''):
    """Sets the information to be displayed on the main page."""

    template_values = self.InitializeTemplate(
        self.APP_NAME + ' Sheriff Rotation Schedule')

    upcoming_schedules = []
    for upcoming_schedule in SheriffSchedules.get_upcoming_schedules():
      schedule_start = upcoming_schedule.schedule_start
      schedule_end = upcoming_schedule.schedule_end
      upcoming_schedule.readable_range = '%s - %s' % (
          schedule_start.strftime('%d %B'), schedule_end.strftime('%d %B'))
      upcoming_schedules.append(upcoming_schedule)

    if upcoming_schedules:
      # The first in the list is the current week.
      upcoming_schedules[0].current_week = True
    # Set the schedules to the template
    template_values['schedules'] = upcoming_schedules
    self.DisplayTemplate('sheriffs.html', template_values)


class update_sheriffs_schedule(BasePage):
  """update_sheriffs_schedule - Sets the sheriff schedule.

  Usage:
    update_sheriffs_schedules?schedule_start=3/11/2013&weeks=10
  The above will populate the schedule for 10 weeks starting on March 11th using
  all the sheriffs in a round robin manner from the Sheriffs table.
  """

  @utils.admin_only
  @utils.require_user
  def get(self):
    return self._handle()

  def _handle(self, error_message='', last_message=''):
    """Sets the information to be displayed on the main page."""
    # Read the schedule_start and weeks URL get parameters.
    schedule_start_tokens = self.request.get('schedule_start').split('/')
    schedule_start = datetime.datetime(int(schedule_start_tokens[2]),
                                       int(schedule_start_tokens[0]),
                                       int(schedule_start_tokens[1]))
    weeks = int(self.request.get('weeks'))

    # Get the list of sheriffs from the Sheriffs table.
    sheriffs = []
    for sheriff in Sheriffs.get_all_sheriffs():
      sheriffs.append(sheriff.username)

    # Delete all entries greater than the specified start time.
    SheriffSchedules.delete_schedules_after(schedule_start)

    # Populate the specified weeks with the sheriffs.
    sheriffs_index = 0
    while weeks > 0:
      curr_schedule_end = schedule_start + datetime.timedelta(days=6)
      SheriffSchedules(
        schedule_start=schedule_start,
        schedule_end=curr_schedule_end,
        username=sheriffs[sheriffs_index]).put()

      # Treat sheriffs like a circular array.
      sheriffs_index = (sheriffs_index + 1) % len(sheriffs)
      # Get the new schedule_start datetime.
      schedule_start = schedule_start + datetime.timedelta(days=7)
      # Decrement the number of weeks left to be filled.
      weeks -= 1


def bootstrap():
  # Guarantee that at least one instance exists.
  if db.GqlQuery('SELECT __key__ FROM Sheriffs').get() is None:
    Sheriffs(username='None').put()

  if db.GqlQuery('SELECT __key__ FROM SheriffSchedules').get() is None:
    SheriffSchedules(
        schedule_start=datetime.datetime(1970, 1, 1),
        schedule_end=datetime.datetime(1970, 1, 7),
        username='None').put()


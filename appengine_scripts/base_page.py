# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility base class."""

import datetime
import hashlib
import logging
import os
import re

from google.appengine.api import memcache
from google.appengine.api import oauth
from google.appengine.api import users
from google.appengine.ext import db
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template

import utils


class Passwords(db.Model):
  """Super users. Useful for automated scripts."""
  password_sha1 = db.StringProperty(required=True, multiline=False)


class GlobalConfig(db.Model):
  """Instance-specific config like application name."""
  app_name = db.StringProperty(required=True)


class BasePage(webapp.RequestHandler):
  """Utility functions needed to validate user and display a template."""

  # Check if the username ends with @chromium.org/@google.com.
  _VALID_EMAIL = re.compile(r"^.*@(chromium\.org|google\.com)$")

  # The name of the application's project.
  APP_NAME = 'Skia'

  def __init__(self, *args, **kwargs):
    super(BasePage, self).__init__(*args, **kwargs)
    self._is_admin = None
    self._user = None
    self._initialized = False

  def _late_init(self):
    """Initializes self._is_admin and self._user once the request object is
    setup.
    """
    self._is_admin = False

    def look_for_password():
      """Looks for password parameter. Not awesome."""
      password = self.request.get('password')
      if password:
        sha1_pass = hashlib.sha1(password).hexdigest()
        if Passwords.gql('WHERE password_sha1 = :1', sha1_pass).get():
          # The password is valid, this is a super admin.
          self._is_admin = True
        else:
          if utils.is_dev_env() and password == 'foobar':
            # Dev server is unsecure.
            self._is_admin = True
          else:
            logging.error('Password is invalid')

    self._user = users.get_current_user()
    if utils.is_dev_env():
      look_for_password()
    elif not self._user:
      try:
        self._user = oauth.get_current_user()
      except oauth.OAuthRequestError:
        if self.request.scheme == 'https':
          look_for_password()

    if not self._is_admin and self._user:
      self._is_admin = bool(
          users.is_current_user_admin() or
          self._VALID_EMAIL.match(self._user.email()))
    self._initialized = True
    logging.info('Admin: %s, User: %s' % (self._is_admin, self._user))

  @property
  def is_admin(self):
    if not self._initialized:
      self._late_init()
    return self._is_admin

  @property
  def user(self):
    if not self._initialized:
      self._late_init()
    return self._user

  def InitializeTemplate(self, title):
    """Initializes the template values with information needed by all pages."""
    if self.user:
      user_email = self.user.email()
    else:
      user_email = ''
    template_values = {
      'app_name': self.APP_NAME,
      'username': user_email,
      'title': title,
      'current_UTC_time': datetime.datetime.now(),
      'is_admin': self.is_admin,
      'user': self.user,
    }
    return template_values

  def DisplayTemplate(self, name, template_values, use_cache=False):
    """Replies to a http request with a template.

    Optionally cache it for 1 second. Only to be used for user-invariant
    pages!
    """
    self.response.headers['Cache-Control'] =  'no-cache, private, max-age=0'
    buff = None
    if use_cache:
      buff = memcache.get(name)
    if not buff:
      path = os.path.join(os.path.dirname(__file__), 'templates/%s' % name)
      buff = template.render(path, template_values)
      if use_cache:
        memcache.add(name, buff, 1)
    self.response.out.write(buff)


def bootstrap():
  if db.GqlQuery('SELECT __key__ FROM Passwords').get() is None:
    # Insert a dummy Passwords so it can be edited through the admin console
    Passwords(password_sha1='invalidhash').put()


# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Commit queue status."""

import cgi
import datetime
import json
import logging
import re
import sys
import urllib2

from google.appengine.api import memcache
from google.appengine.api import users
from google.appengine.ext import db
from google.appengine.ext.db import polymodel

from base_page import BasePage
import utils


TRY_SERVER_MAP = (
  'SUCCESS', 'WARNINGS', 'FAILURE', 'SKIPPED', 'EXCEPTION', 'RETRY',
)

# Verification name in commit-queue/verification/*.py. Initialized at the bottom
# of this file.
EVENT_MAP = {}


class Owner(db.Model):
  """key == email address."""
  email = db.EmailProperty()

  @staticmethod
  def to_key(owner):
    return '<%s>' % owner


class PendingCommit(db.Model):
  """parent is Owner."""
  created = db.DateTimeProperty()
  done = db.BooleanProperty(default=False)
  issue = db.IntegerProperty()
  patchset = db.IntegerProperty()

  @staticmethod
  def to_key(issue, patchset, owner):
    # TODO(maruel): My bad, shouldn't have put owner in the key.
    return '<%d-%d-%s>' % (issue, patchset, owner)


class VerificationEvent(polymodel.PolyModel):
  """parent is PendingCommit."""
  created = db.DateTimeProperty(auto_now_add=True)
  result = db.IntegerProperty()
  timestamp = db.DateTimeProperty()

  @property
  def as_html(self):
    raise NotImplementedError()


class TryServerEvent(VerificationEvent):
  name = 'try server'
  build = db.IntegerProperty()
  builder = db.StringProperty()
  clobber = db.BooleanProperty()
  job_name = db.StringProperty()
  revision = db.IntegerProperty()
  url = db.StringProperty()

  @property
  def as_html(self):
    if self.build is not None:
      out = '<a href="%s">"%s" on %s, build #%s</a>' % (
          cgi.escape(self.url),
          cgi.escape(self.job_name),
          cgi.escape(self.builder),
          cgi.escape(str(self.build)))
      if (self.result is not None and
          0 <= self.result < len(TRY_SERVER_MAP[self.result])):
        out = '%s - result: %s' % (out, TRY_SERVER_MAP[self.result])
      return out
    else:
      # TODO(maruel): Load the json
      # ('http://build.chromium.org/p/tryserver.chromium/json/builders/%s/'
      #  'pendingBuilds') % self.builder and display the rank.
      return '"%s" on %s (pending)' % (
          cgi.escape(self.job_name),
          cgi.escape(self.builder))

  @classmethod
  def to_key(cls, packet):
    if not packet.get('builder') or not packet.get('job_name'):
      return None
    return '<%s-%s-%s>' % (
        cls.name, packet['builder'], packet['job_name'])


class TryJobRietveldEvent(VerificationEvent):
  """Same thing as TryServerEvent. Should probably be kept in sync with
  TryServerEvent.

  It comes from commit-queue/verification/try_job_on_rietveld.py.
  """
  name = 'try job rietveld'
  build = db.IntegerProperty()
  builder = db.StringProperty()
  clobber = db.BooleanProperty()
  job_name = db.StringProperty()
  # TODO(maruel): Transition all revision properties to string, since it could
  # be a hash for git commits.
  revision = db.StringProperty()
  url = db.StringProperty()

  @property
  def as_html(self):
    if self.build is not None:
      out = '<a href="%s">"%s" on %s, build #%s</a>' % (
          cgi.escape(self.url),
          cgi.escape(self.job_name),
          cgi.escape(self.builder),
          cgi.escape(str(self.build)))
      if (self.result is not None and
          0 <= self.result < len(TRY_SERVER_MAP[self.result])):
        out = '%s - result: %s' % (out, TRY_SERVER_MAP[self.result])
      return out
    else:
      # TODO(maruel): Load the json
      # ('http://build.chromium.org/p/tryserver.chromium/json/builders/%s/'
      #  'pendingBuilds') % self.builder and display the rank.
      return '"%s" on %s (pending)' % (
          cgi.escape(self.job_name),
          cgi.escape(self.builder))

  @classmethod
  def to_key(cls, packet):
    if not packet.get('builder') or not packet.get('job_name'):
      return None
    return '<%s-%s-%s>' % (
        cls.name, packet['builder'], packet['job_name'])


class PresubmitEvent(VerificationEvent):
  name = 'presubmit'
  duration = db.FloatProperty()
  output = db.TextProperty()
  timed_out = db.BooleanProperty()

  @property
  def as_html(self):
    return '<pre class="output">%s</pre>' % cgi.escape(self.output)

  @classmethod
  def to_key(cls, _):
    # There shall be only one PresubmitEvent per PendingCommit.
    return '<%s>' % cls.name


class CommitEvent(VerificationEvent):
  name = 'commit'
  output = db.TextProperty()
  revision = db.IntegerProperty()
  url = db.StringProperty()

  @property
  def as_html(self):
    out = '<pre class="output">%s</pre>' % cgi.escape(self.output)
    if self.url:
      out += '<a href="%s">Revision %s</a>' % (
        cgi.escape(self.url),
        cgi.escape(str(self.revision)))
    elif self.revision:
      out += '<br>Revision %s' % cgi.escape(str(self.revision))
    return out

  @classmethod
  def to_key(cls, _):
    return '<%s>' % cls.name


class InitialEvent(VerificationEvent):
  name = 'initial'
  revision = db.IntegerProperty()

  @property
  def as_html(self):
    return 'Looking at new commit, using revision %s' % (
        cgi.escape(str(self.revision)))

  @classmethod
  def to_key(cls, _):
    return '<%s>' % cls.name


class AbortEvent(VerificationEvent):
  name = 'abort'
  output = db.TextProperty()

  @property
  def as_html(self):
    return '<pre class="output">%s</pre>' % cgi.escape(self.output)

  @classmethod
  def to_key(cls, _):
    return '<%s>' % cls.name


class WhyNotEvent(VerificationEvent):
  name = 'why not'
  message = db.TextProperty()

  @property
  def as_html(self):
    return '<pre class="output">%s</pre>' % cgi.escape(self.message)

  @classmethod
  def to_key(cls, _):
    return '<%s>' % cls.name


def get_owner(owner):
  """Efficient querying of Owner with memcache."""
  key = Owner.to_key(owner)
  obj = memcache.get(key, namespace='Owner')
  if not obj:
    obj = Owner.get_or_insert(key_name=key, email=owner)
    memcache.set(key, obj, time=60*60, namespace='Owner')
  return obj


def get_pending_commit(issue, patchset, owner, timestamp):
  """Efficient querying of PendingCommit with memcache."""
  owner_obj = get_owner(owner)
  key = PendingCommit.to_key(issue, patchset, owner)
  obj = memcache.get(key, namespace='PendingCommit')
  if not obj:
    obj = PendingCommit.get_or_insert(
        key_name=key, parent=owner_obj, issue=issue, patchset=patchset,
        owner=owner, created=timestamp)
    memcache.set(key, obj, time=60*60, namespace='PendingCommit')
  return obj


class CQBasePage(BasePage):
  """Returns a web page or json data about commit queue state.

  Can filter for everyone, a particular user or a particular issue.

  Query args:
  - format: can be 'html' or 'json'.
  - limit: maximum number of elements to result. default is 100.
  """

  def get(self, *args):
    query = self._get_query(*args)
    if not query:
      # The user probably used /me without being logged.
      self.redirect(users.create_login_url(self.request.url))
      return
    out_format = self.request.get('format', 'html')
    if out_format == 'json':
      return self._get_as_json(query)
    else:
      return self._get_as_html(query)

  def _get_query(self, owner=None, issue=None, patchset=None):
    """Returns None on query failure."""
    query = VerificationEvent.all().order('-timestamp')
    ancestor = None
    if owner:
      owner = self._parse_user(owner)
      if not owner:
        return None
      ancestor = db.Key.from_path('Owner', Owner.to_key(owner))

    if issue:
      issue = int(issue)
      if patchset:
        patchset = int(patchset)
        pending_key = PendingCommit.to_key(issue, patchset, owner)
        ancestor = db.Key.from_path(
            'PendingCommit', pending_key, parent=ancestor)
      else:
        # Only show the last object since it's complex to do a OR with multiple
        # ancestors.
        ancestor = db.Query(PendingCommit, keys_only=True).filter(
            'issue =', issue).ancestor(ancestor).order('-created').get()

    if ancestor:
      query.ancestor(ancestor)
    return query

  def _get_limit(self):
    limit = self.request.get('limit')
    if limit and limit.isdigit():
      limit = int(limit)
    else:
      limit = 100
    return limit

  def _get_as_json(self, query):
    self.response.headers['Content-Type'] = 'application/json'
    self.response.headers['Access-Control-Allow-Origin'] = '*'
    data = json.dumps([s.AsDict() for s in query.fetch(self._get_limit())])
    callback = self.request.get('callback')
    if callback:
      if re.match(r'^[a-zA-Z$_][a-zA-Z$0-9._]*$', callback):
        data = '%s(%s);' % (callback, data)
    self.response.out.write(data)

  def _get_as_html(self, query):
    raise NotImplementedError()

  def _parse_user(self, user):
    user = urllib2.unquote(user.strip('/'))
    if user == 'me':
      if not self.user:
        user = None
      else:
        user = self.user.email()
    return user


class OwnerStats(object):
  """CQ usage statistics for a single user."""
  def __init__(self, now, owner, last_day, last_week, last_month, forever):
    # Since epoch in float.
    self.now = now
    # User instance.
    self.owner = owner
    assert all(isinstance(i, PendingCommit) for i in last_day)
    self.last_day = last_day
    assert all(isinstance(i, PendingCommit) for i in last_week)
    self.last_week = last_week
    assert isinstance(last_month, int)
    self.last_month = last_month
    assert isinstance(forever, int)
    self.forever = forever
    # Gamify ALL the things!
    self.points = (
        len(self.last_day) * 10 +
        len(self.last_week) * 5 +
        self.last_month * 2 +
        self.forever)


class OwnerQuery(object):
  def __init__(self, owner_key, now):
    self.owner_key = owner_key
    self.now = now
    since = lambda x: now - datetime.timedelta(days=x)
    self._owner = db.get_async(owner_key)
    self._last_day = self._pendings().filter('created >=', since(1)).run()
    self._last_week = self._pendings().filter(
        'created >=', since(7)).filter('created <', since(1)).run()
    # These block.
    self.last_month = self._pendings().filter(
        'created >=', since(30)).count()
    self.forever = self._pendings(keys_only=True).count()

  def _pendings(self, **kwargs):
    return PendingCommit.all(**kwargs).ancestor(self.owner_key)

  def to_stats(self):
    obj = OwnerStats(
        self.now,
        self._owner.get_result(),
        list(self._last_day),
        list(self._last_week),
        self.last_month,
        self.forever)
    memcache.add(
        self.owner_key.name(), obj, 2*60*60, namespace='cq_owner_stats')
    return obj


def to_link(pending):
  return '<a href="/cq/%s/%s/%s">%s</a>' % (
      pending.parent_key().name()[1:-1],
      pending.issue,
      pending.patchset,
      pending.issue)


def get_owner_stats(owner_key, now):
  """Returns an OnwerStats instance for the Owner."""
  obj = memcache.get(owner_key.name(), 'cq_owner_stats')
  if obj:
    return obj
  return OwnerQuery(owner_key, now).to_stats()


def monthly_top_contributors():
  """Returns the top monthly contributors as a list of OwnerStats."""
  obj = memcache.get('monthly', 'cq_top')
  if not obj:
    now = datetime.datetime.utcnow()
    last_pendings = PendingCommit.all(
        keys_only=True).order('-created').fetch(1000)
    # Make it use asynchronous queries.
    obj = [
        get_owner_stats(o, now) for o in set(p.parent() for p in last_pendings)
    ]
    memcache.add('monthly', obj, 2*60*60, namespace='cq_top')
  return obj


class Summary(CQBasePage):
  def _get_as_html(self, _):
    owners = []
    for stats in monthly_top_contributors():
      data = {
        'email': stats.owner.email,
        'last_day': ', '.join(to_link(i) for i in stats.last_day),
        'last_week': ', '.join(to_link(i) for i in stats.last_week),
        'last_month': stats.last_month,
        'forever': stats.forever,
      }
      owners.append(data)
    owners.sort(key=lambda x: -x['last_month'])
    template_values = self.InitializeTemplate(self.APP_NAME + ' Commit queue')
    template_values['data'] = owners
    self.DisplayTemplate('cq_owners.html', template_values, use_cache=True)


class TopScore(CQBasePage):
  def _get_as_html(self, _):
    owners = [
      {
        'name': stats.owner.email.split('@', 1)[0].upper(),
        'points': stats.points,
      }
      for stats in monthly_top_contributors()
    ]
    owners.sort(key=lambda x: -x['points'])
    for i in xrange(len(owners)):
      if i == 0:
        owners[i]['rank'] = '1st'
      elif i == 1:
        owners[i]['rank'] = '2nd'
      elif i == 2:
        owners[i]['rank'] = '3rd'
      else:
        owners[i]['rank'] = '%dth' % (i + 1)
    template_values = self.InitializeTemplate(self.APP_NAME + ' Commit queue')
    template_values['data'] = owners
    self.DisplayTemplate('cq_top_score.html', template_values, use_cache=True)


class User(CQBasePage):
  def _get_as_html(self, query):
    pending_commits_events = {}
    pending_commits = {}
    for event in query.fetch(self._get_limit()):
      # Implicitly find PendingCommit's.
      pending_commit = event.parent()
      if not pending_commit:
        logging.warn('Event %s is corrupted, can\'t find %s' % (
          event.key().id_or_name(), event.parent_key().id_or_name()))
        continue
      pending_commits_events.setdefault(pending_commit.key(), []).append(event)
      pending_commits[pending_commit.key()] = pending_commit

    sorted_data = []
    for pending_commit in sorted(
        pending_commits.itervalues(), key=lambda x: x.created, reverse=True):
      sorted_data.append(
          (pending_commit,
            reversed(pending_commits_events[pending_commit.key()])))
    template_values = self.InitializeTemplate(self.APP_NAME + ' Commit queue')
    template_values['data'] = sorted_data
    self.DisplayTemplate('cq_owner.html', template_values, use_cache=True)


class Issue(CQBasePage):
  def _get_as_html(self, query):
    pending_commits_events = {}
    pending_commits = {}
    for event in query.fetch(self._get_limit()):
      # Implicitly find PendingCommit's.
      pending_commit = event.parent()
      if not pending_commit:
        logging.warn('Event %s is corrupted, can\'t find %s' % (
          event.key().id_or_name(), event.parent_key().id_or_name()))
        continue
      pending_commits_events.setdefault(pending_commit.key(), []).append(event)
      pending_commits[pending_commit.key()] = pending_commit

    sorted_data = []
    for pending_commit in sorted(
        pending_commits.itervalues(), key=lambda x: x.issue):
      sorted_data.append(
          (pending_commit,
            reversed(pending_commits_events[pending_commit.key()])))
    template_values = self.InitializeTemplate(self.APP_NAME + ' Commit queue')
    template_values['data'] = sorted_data
    self.DisplayTemplate('cq_owner.html', template_values, use_cache=True)


class Receiver(BasePage):
  @utils.admin_only
  def post(self):
    def load_values():
      for p in self.request.get_all('p'):
        try:
          yield json.loads(p)
        except ValueError:
          logging.warn('Discarding invalid packet %r' % p)

    count = 0
    for packet in load_values():
      cls = EVENT_MAP.get(packet.get('verification'))
      if (not cls or
          not isinstance(packet.get('issue'), int) or
          not isinstance(packet.get('patchset'), int) or
          not packet.get('timestamp') or
          not isinstance(packet.get('owner'), basestring)):
        logging.warning('Ignoring packet %s' % packet)
        continue

      payload = packet.get('payload', {})
      # TODO(maruel): Convert the type implicitly, because storing a int into a
      # FloatProperty or a StringProperty will raise a BadValueError.
      values = dict(
          (i, payload[i]) for i in cls.properties()
          if i not in ('_class', 'pending') and i in payload)
      # Inject the timestamp.
      values['timestamp'] = datetime.datetime.utcfromtimestamp(
          packet['timestamp'])
      pending = get_pending_commit(
          packet['issue'], packet['patchset'], packet['owner'],
          values['timestamp'])

      logging.debug('New packet %s' % cls.__name__)
      key_name = cls.to_key(values)
      if not key_name:
        continue

      # TODO(maruel) Use an async transaction, in batch.
      obj = cls.get_by_key_name(key_name, parent=pending)
      # Compare the timestamps. Events could arrive in the reverse order.
      if not obj or obj.timestamp <= values['timestamp']:
        # This will override the previous obj if it existed.
        cls(parent=pending, key_name=key_name, **values).put()
        count += 1
      elif obj:
        logging.warn('Received object out of order')

      # Cache the fact that the change was committed in the PendingCommit.
      if packet['verification'] == 'commit':
        pending.done = True
        pending.put()

    self.response.out.write('%d\n' % count)


def bootstrap():
  # Used by _parse_packet() to find the right model to use from the
  # 'verification' value of the packet.
  module = sys.modules[__name__]
  for i in dir(module):
    if i.endswith('Event') and i != 'VerificationEvent':
      obj = getattr(module, i)
      EVENT_MAP[obj.name] = obj

# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Scheduler which supports dependency chaining."""


from twisted.internet import defer
from twisted.python import log
from buildbot import util
from buildbot.db import buildsets
from buildbot.status.results import SUCCESS, WARNINGS
from buildbot.schedulers import base, triggerable

import copy


# Monkeypatch this functionality into the buildsets connector.
def BuildsetsConnectorComponentGetBuildsetsForSourceStamp(self, ssid):
  """Retrieve all Buildsets for the given Source Stamp.

  Args:
      ssid: ID of the Source Stamp in question.

  Returns:
      List of Buildset dicts associated with the given Source Stamp.
  """
  def thd(conn):
    bs_tbl = self.db.model.buildsets
    q = bs_tbl.select()
    q = q.where(bs_tbl.c.sourcestampid == ssid)
    res = conn.execute(q)
    return [ self._row2dict(row) for row in res.fetchall() ]
  return self.db.pool.do(thd)
buildsets.BuildsetsConnectorComponent.getBuildsetsForSourceStamp = \
    BuildsetsConnectorComponentGetBuildsetsForSourceStamp


class DependencyChainScheduler(base.BaseScheduler):
  """Dependent-like scheduler which attempts to satisfy its own dependencies,
  rather than waiting for them to be satisfied."""
  # TODO(borenet): This class needs more work, particularly the pending dict,
  # which will grow arbitrarily large as more builds come in and will not
  # persist across master restarts. Instead, we should use a new database table.

  compare_attrs = base.BaseScheduler.compare_attrs + ('dependencies', 'pending')

  def __init__(self, dependencies, name, builder_name, properties=None):
    """Initialize this DependencyChainScheduler.

    Args:
        dependencies: List of DependencyChainScheduler instances whose builds
            must be run before this DependencyChainScheduler's builds may run.
        name: Name of this DependencyChainScheduler.
        builder_name: Name of the (single) builder triggered by this Scheduler.
        properties: Optional dictionary of build properties which builds
            triggered by this Scheduler will inherit.
    """
    if not properties:
      properties = {}
    base.BaseScheduler.__init__(self, name, [builder_name], properties)
    for dep in dependencies:
      assert isinstance(dep, DependencyChainScheduler)
    self.dependencies = dependencies
    self.pending = {}
    self._buildset_addition_subscr = None
    self._buildset_completion_subscr = None

    # the subscription lock makes sure that we're done inserting a
    # subcription into the DB before registering that the buildset is
    # complete.
    self._subscription_lock = defer.DeferredLock()

    self._buildset_lock = defer.DeferredLock()

  @util.deferredLocked('_buildset_lock')
  def maybeAddBuildsetForSourceStamp(self, launch_dependencies, ssid, **kwargs):
    """If all dependencies have been met for the given Source Stamp, add build
    sets for each pending build for the given Source Stamp.

    Args:
        launch_dependencies: Boolean; whether or not the dependencies for this
            DependencyChainScheduler should be triggered if they haven't yet
            been satisfied.
        ssid: ID of a Source Stamp.
    """
    unmet_dependencies = []
    dep_failed = False
    for dep in self.dependencies:
      dep_results = self.pending[ssid]['dependencies'].get(dep.name)
      if dep_results is None:
        # No build exists for this ssid. Launch one.
        unmet_dependencies.append(dep)
      elif dep_results not in (SUCCESS, WARNINGS):
        # Build failed for this ssid. Abort.
        print 'Dependency %s failed for %s. Not starting build.' % (dep.name,
                                                                    self.name)
        dep_failed = True

    if dep_failed:
      # Abort all pending builds, since their dependencies can't be satisfied.
      while self.pending[ssid]['pending']:
        print 'Aborting: %s' % self.pending[ssid]['pending'].pop()
      return

    if unmet_dependencies:
      if launch_dependencies:
        print '%s attempting to satisfy: %s' % (
            self.name, [dep.name for dep in unmet_dependencies])
        dl = [dep.addBuildsetForSourceStamp(ssid, **kwargs)
              for dep in unmet_dependencies]
        return defer.DeferredList(dl)
    else:
      dl = []
      while self.pending[ssid]['pending']:
        print '%s adding buildset for %d' % (self.name, ssid)
        kwargs = self.pending[ssid]['pending'].pop()
        dl.append(base.BaseScheduler.addBuildsetForSourceStamp(self, ssid=ssid,
                                                               **kwargs))
      return defer.DeferredList(dl)

  def addBuildsetForSourceStamp(self, ssid, **kwargs):
    """Attempt to add a buildset for the given Source Stamp.

    For other Schedulers this function just adds the buildset. Instead, this
    Scheduler adds an entry to the pending dictionary and then calls
    maybeAddBuildsetForSourceStamp, which will launch any unmet dependencies or
    add the buildset.

    Args:
        ssid: ID of a Source Stamp.
    """
    print '%s: addBuildsetForSourceStamp' % self.name

    if not self.pending.get(ssid):
      self.pending[ssid] = {
          'dependencies': {},
          'pending': [],
      }

    def _got_buildset_props(props, buildsets):
      """Callback which runs once the properties for a list of buildsets have
      been retrieved from the database. Determines whether a buildset already
      exists for the given Source Stamp and if not, calls
      maybeAddBuildsetForSourceStamp to add the buildset or satisfy its
      dependencies.

      Args:
          props: List of tuples of the form: (bool, dict) where each dict
              contains the properties, including the scheduler name, for a
              buildset.
          buildsets: List of buildset dictionaries.
      """
      assert len(buildsets) == len(props)
      already_have_buildset = False
      for i in xrange(len(buildsets)):
        print 'considering buildset for %s' % str(props[i][1])
        if props[i][1].get('scheduler', (None, None))[0] == self.name:
          already_have_buildset = True
          print '%s already has buildset for %d' % (self.name, ssid)
          break
      if (kwargs not in self.pending[ssid]['pending']
          and not already_have_buildset):
        print '%s adding pending build for %d (addBuildsetforSourceStamp)' % (
            self.name, ssid or -1)
        self.pending[ssid]['pending'].append(kwargs)
      else:
        print '%s NOT adding duplicate build for %d' % (self.name, ssid)

      return self.maybeAddBuildsetForSourceStamp(
          launch_dependencies=True, ssid=ssid, **kwargs)

    def _got_buildsets(buildsets):
      """Callback which runs once a list of buildsets have been retrieved from
      the database. Gets the properties for each buildset from the database and
      chains a callback.

      Args:
          buildsets: List of buildset dictionaries.
      """
      dl = []
      for buildset in buildsets:
        dl.append(
            self.master.db.buildsets.getBuildsetProperties(buildset['bsid']))
      d = defer.DeferredList(dl)
      d.addCallback(_got_buildset_props, buildsets)
      return d
    d = self.master.db.buildsets.getBuildsetsForSourceStamp(ssid)
    d.addErrback(log.err, 'while getting buildsets for %d' % ssid)
    d.addCallback(_got_buildsets)
    return d

  def startService(self):
    """Called when the DependencyChainScheduler starts running. Registers
    callback functions for when buildsets are added or completed."""
    self._buildset_addition_subscr = \
            self.master.subscribeToBuildsets(self._buildsetAdded)
    self._buildset_completion_subscr = \
            self.master.subscribeToBuildsetCompletions(self._buildsetCompleted)
    # check for any buildsets completed before we started
    d = self._checkCompletedBuildsets(None, None)
    d.addErrback(log.err, 'while checking for completed buildsets in start')

  def stopService(self):
    """Called when the DependencyChainScheduler stops running. Unregisters all
    callback functions."""
    if self._buildset_addition_subscr:
        self._buildset_addition_subscr.unsubscribe()
    if self._buildset_completion_subscr:
        self._buildset_completion_subscr.unsubscribe()
    return defer.succeed(None)

  @util.deferredLocked('_subscription_lock')
  def _buildsetAdded(self, bsid=None, properties=None, **kwargs):
    """Callback function which runs when any buildset is added on the current
    build master. Determines whether the buildset belongs to one of our
    dependencies, and subscribes to that buildset if so.

    Args:
        bsid: ID of the added buildset.
        properties: dictionary of properties of the added buildset.
    """
    submitter = properties.get('scheduler', (None, None))[0]
    buildset_is_relevant = False
    for dep in self.dependencies:
      if submitter == dep.name:
        buildset_is_relevant = True
        break
    if not buildset_is_relevant:
      return

    d = self.master.db.buildsets.subscribeToBuildset(
                                    self.schedulerid, bsid)
    d.addErrback(log.err, 'while subscribing to buildset %d' % bsid)

  def _buildsetCompleted(self, bsid, result):
    """Callback function which runs when any buildset is completed on the
    current build master. Just runs the _checkCompletedBuildsets function to
    determine whether any action is necessary.

    Args:
        bsid: ID of the completed buildset.
        result: Result of the completed buildset. Possible values are defined in
            builbot.status.results.
    """
    d = self._checkCompletedBuildsets(bsid, result)
    d.addErrback(log.err, 'while checking for completed buildsets')

  @util.deferredLocked('_subscription_lock')
  @defer.deferredGenerator
  def _checkCompletedBuildsets(self, bsid, result):
    """For each newly-completed buildset, determine whether the buildset
    satisfied a dependency for a Source Stamp. If so, run
    maybeAddBuildsetForSourceStamp which will add a buildset for the source
    stamp if all of its dependencies have been satisfied.

    Args:
        bsid: ID of the completed buildset.
        result: Result of the completed buildset. Possible values are defined in
            builbot.status.results.
    """
    wfd = defer.waitForDeferred(
        self.master.db.buildsets.getSubscribedBuildsets(self.schedulerid))
    yield wfd
    subscribed_buildsets = wfd.getResult()

    finished_ssids = set()
    for (sub_bsid, sub_ssid, sub_complete, sub_results) in subscribed_buildsets:
      # skip incomplete builds, handling the case where the 'complete'
      # column has not been updated yet
      if not sub_complete and sub_bsid != bsid:
        continue

      print '%s: subscribed buildset finished: %d' % (self.name, sub_bsid or -1)

      # Unsubscribe from the buildset.
      wfd = defer.waitForDeferred(
          self.master.db.buildsets.unsubscribeFromBuildset(self.schedulerid,
                                                           bsid))
      yield wfd
      wfd.getResult()

      # Get the scheduler name.
      wfd = defer.waitForDeferred(
          self.master.db.buildsets.getBuildsetProperties(bsid))
      yield wfd
      build_set_props = wfd.getResult()
      if not build_set_props.get('scheduler'):
        continue
      scheduler_name = str(build_set_props['scheduler'][0])

      if not self.pending.get(sub_ssid):
        self.pending[sub_ssid] = {
            'dependencies': {},
            'pending': [],
        }
      self.pending[sub_ssid]['dependencies'][scheduler_name] = sub_results
      finished_ssids.add(sub_ssid)

    for finished_ssid in finished_ssids:
      print '%s running maybeAddBuildsetForSourceStamp(%d); pending: %s' % (
          self.name, finished_ssid, self.pending[finished_ssid]['pending'])
      self.maybeAddBuildsetForSourceStamp(launch_dependencies=False,
                                          ssid=finished_ssid)

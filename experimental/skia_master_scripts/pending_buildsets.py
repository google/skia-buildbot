# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Database connector component for not-yet-added buildsets."""


from buildbot.db import base
from buildbot.util import epoch2datetime
from twisted.internet import defer
from twisted.internet import reactor

import ast
import sqlalchemy as sa


class PendingBuildsetsConnectorComponent(base.DBConnectorComponent):
  """A DBConnectorComponent for the pending buildsets table, which tracks
  buildsets which have not yet been inserted into the primary buildsets table.
  """

  def add_pending_buildset(self, ssid, scheduler, _reactor=reactor, **kwargs):
    """Add a new Buildset to the pending buildsets table.

    Args:
        ssid: The ID of the SourceStamp which triggered this Buildset.
        scheduler: Name of the Scheduler requesting this Buildset.
        kwargs: Extra arguments. These get stored with the Buildset and are used
            when adding it to the active buildsets table.
        _reactor: reactor module, for testing
    """
    bs_args = dict(kwargs)
    if 'properties' in bs_args.keys():
      properties = bs_args.pop('properties')
    else:
      properties = {}
    def thd(conn):
      submitted_at = _reactor.seconds()
      table = self.db.model.pending_buildsets
      query = table.insert()
      result = conn.execute(query, dict(
          sourcestampid=ssid,
          submitted_at=submitted_at,
          scheduler=scheduler,
          properties=str(properties),
          **bs_args
      ))
      return result.inserted_primary_key[0]

    return self.db.pool.do(thd)

  def get_pending_buildsets(self, ssid, scheduler):
    """Get all pending Buildsets for the given Scheduler and Source Stamp.

    Args:
        ssid: ID of a Source Stamp.
        scheduler: Name of a scheduler

    Returns:
        List of dictionaries where each dictionary is the kwargs which were
        provided to add_pending_buildset when that Buildset was added.
    """
    def thd(conn):
      table = self.db.model.pending_buildsets
      query = table.select()
      query = query.where((table.c.scheduler == scheduler) &
                          (table.c.sourcestampid == ssid))
      result = conn.execute(query)
      return [self._row2dict(row) for row in result.fetchall()]

    return self.db.pool.do(thd)

  def cancel_pending_buildsets(self, ssid, scheduler):
    """Remove all pending Buildsets for the given Scheduler and Source Stamp.

    Args:
        ssid: ID of a Source Stamp.
        scheduler: Name of a scheduler

    Returns:
        List of dictionaries for the removed Buildsets where each dictionary is
        the kwargs which were provided to add_pending_buildset when that
        Buildset was added.
    """
    def thd(conn):
      table = self.db.model.pending_buildsets
      query = table.select()
      query = query.where((table.c.scheduler == scheduler) &
                          (table.c.sourcestampid == ssid))
      result = conn.execute(query)
      canceled_buildsets = [self._row2dict(row) for row in result.fetchall()]
      query = table.delete()
      query = query.where((table.c.scheduler == scheduler) &
                          (table.c.sourcestampid == ssid))
      conn.execute(query)
      return canceled_buildsets

    return self.db.pool.do(thd)

  def _row2dict(self, row):
    def mkdt(epoch):
      if epoch:
        return epoch2datetime(epoch)
    return dict(external_idstring=row.external_idstring,
        reason=row.reason, sourcestampid=row.sourcestampid,
        submitted_at=mkdt(row.submitted_at),
        complete=bool(row.complete),
        complete_at=mkdt(row.complete_at), results=row.results,
        bsid=row.id,
        properties=ast.literal_eval(row.properties))

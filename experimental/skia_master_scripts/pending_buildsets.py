# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Database connector component for not-yet-added buildsets."""


from twisted.internet import defer

import sqlalchemy as sa


# TODO(borenet): Replace this and change this module to use the database.
# Global pending buildsets dictionary.
_data = {}


class PendingBuildsetsConnectorComponent():
  """A DBConnectorComponent for the pending buildsets table, which tracks
  buildsets which have not yet been inserted into the primary buildsets table.
  """

  def add_pending_buildset(self, ssid, scheduler, **kwargs):
    """Add a new Buildset to the pending buildsets table.

    Args:
        ssid: The ID of the SourceStamp which triggered this Buildset.
        scheduler: Name of the Scheduler requesting this Buildset.
        kwargs: Extra arguments. These get stored with the Buildset and are used
            when adding it to the active buildsets table.
    """
    if not _data.get(ssid):
      _data[ssid] = {}
    if not _data[ssid].get(scheduler):
      _data[ssid][scheduler] = []
    _data[ssid][scheduler].append(kwargs)
    return defer.succeed(None)

  def get_pending_buildsets(self, ssid, scheduler):
    """Get all pending Buildsets for the given Scheduler and Source Stamp.

    Args:
        ssid: ID of a Source Stamp.
        scheduler: Name of a scheduler

    Returns:
        List of dictionaries where each dictionary is the kwargs which were
        provided to add_pending_buildset when that Buildset was added.
    """
    return defer.succeed(_data.get(ssid, {}).get(scheduler, []))

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
    pending = _data.get(ssid, {}).get(scheduler, [])
    if _data.get(ssid, {}).get(scheduler):
      _data.get(ssid, {}).pop(scheduler)
    if _data.get(ssid) == {}:
      _data.pop(ssid)
    return defer.succeed(pending)

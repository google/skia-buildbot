# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Skia-specific database setup."""


import sqlalchemy as sa

from buildbot.db import base, connector, enginestrategy, pool
from skia_master_scripts import pending_buildsets
from twisted.application import service
from twisted.internet import defer


class SkiaConnector(service.MultiService):
  """Database connector for the Skia-specific database."""
  def __init__(self, master, db_url, basedir):
    """Initialize the database connector. Sets up the model and a connector to
    the pending_buildsets table.

    Args:
        master: instance of BuildMaster.
        db_url: URL of the database file.
        basedir: directory in which the BuildMaster runs.
    """
    service.MultiService.__init__(self)
    self.master = master
    self.basedir = basedir

    self._engine = enginestrategy.create_engine(db_url, basedir=self.basedir)
    self.pool = pool.DBThreadPool(self._engine)

    self.model = SkiaModel(self)
    self.pending_buildsets = \
        pending_buildsets.PendingBuildsetsConnectorComponent(self)


class SkiaModel(base.DBConnectorComponent):
  """Database model for the Skia-specific database."""

  metadata = sa.MetaData()

  # Not-yet-inserted buildsets.
  pending_buildsets = sa.Table('pending_buildsets', metadata,
      sa.Column('id', sa.Integer,  primary_key=True),
      sa.Column('external_idstring', sa.String(256)),
      sa.Column('reason', sa.String(256)),
      sa.Column('scheduler', sa.String(256)),
      sa.Column('dependencies', sa.String(2048)),
      sa.Column('sourcestampid', sa.Integer, nullable=False),
      sa.Column('submitted_at', sa.Integer, nullable=False),
      sa.Column('complete', sa.SmallInteger, nullable=False,
                server_default=sa.DefaultClause("0")),
      sa.Column('complete_at', sa.Integer),
      sa.Column('results', sa.SmallInteger),
      sa.Column('properties', sa.String(2048))
  )
  sa.Index('pending_buildsets_scheduler', pending_buildsets.c.scheduler)
  sa.Index('pending_buildsets_sourcestampid', pending_buildsets.c.sourcestampid)

  def upgrade_if_needed(self):
    """Ensure that the database is up-to-date. At this time there is only one
    version of the database model, so this function just creates it if needed.
    """
    def table_exists(engine, table):
      """Determine whether the given table exists.

      Args:
          engine: SQLAlchemy database engine instance.
          table: string; name of the table to test.

      Returns:
          True if the table exists and False otherwise.
      """
      try:
        engine.execute('select * from %s limit 1' % table).close()
        return True
      except:
        return False

    def thd(engine):
      """Create the database if it does not already exist."""
      if not table_exists(engine, 'pending_buildsets'):
        self.metadata.bind = engine
        self.metadata.create_all()
    return self.db.pool.do_with_engine(thd)

# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Skia's subclass of BuildMaster."""


from buildbot import master
from skia_master_scripts import db
from twisted.internet import defer


class SkiaMaster(master.BuildMaster):
  """Subclass of buildbot.master.BuildMaster which we use to add an extra
  database which contains not-yet-inserted buildsets."""

  def __init__(self, basedir, configFileName='master.cfg'):
    """Initialize the SkiaMaster.

    Args:
        basedir: directory in which the master runs.
        configFileName: name of the build master config file.
    """
    master.BuildMaster.__init__(self, basedir=basedir,
                                configFileName=configFileName)
    self.skia_db = None

  def loadSkiaDatabase(self, db_url):
    """Initialize the extra, Skia-specific database.

    Args:
        db_url: URL of the master database file. The Skia-specific database just
            appends '_skia' to this URL.
    """
    self.skia_db = db.SkiaConnector(self, db_url + '_skia', self.basedir)
    self.skia_db.setServiceParent(self)
    return self.skia_db.model.upgrade_if_needed()

  def loadConfig_Database(self, db_url, db_poll_interval):
    """Override of BuildMaster.loadConfig_Database which also loads the Skia-
    specific database.

    Args:
        db_url: URL of the master database file. The Skia-specific database just
            appends '_skia' to this URL.
        db_poll_interval: How often to poll the database for changes. The
            BuildMaster uses this in the case of multiple BuildMaster instances
            which share a single database. The Skia-specific database does not
            use this parameter.
    """
    dl = []
    dl.append(self.loadSkiaDatabase(db_url=db_url))
    dl.append(master.BuildMaster.loadConfig_Database(self, db_url=db_url,
        db_poll_interval=db_poll_interval))
    return defer.DeferredList(dl)

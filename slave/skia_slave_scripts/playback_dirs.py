# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Encapsulates all required directories for skp playback (RenderPictures)
BuildSteps."""

import os
import posixpath


# The playback root directory name will be used both locally and on Google
# Storage.
ROOT_PLAYBACK_DIR_NAME = 'playback'
# These subdirectory names will be used both locally and on Google Storage.
SKPICTURES_DIR_NAME = 'skps'
IMAGERESULTS_DIR_NAME = 'imageResults'


class SkpPlaybackDirs(object):
  """Interface for required directories for skp playback BuildSteps."""

  def __init__(self, builder_name, perf_output_basedir):
    """Create an instance of SkpPlaybackDirs."""
    self._builder_name = builder_name
    self._perf_output_basedir = perf_output_basedir

  def PlaybackRootDir(self):
    raise NotImplementedError("PlaybackRootDir is unimplemented!")

  def PlaybackSkpDir(self):
    raise NotImplementedError("PlaybackSkpDir is unimplemented!")

  def PlaybackImageResultsDir(self):
    raise NotImplementedError("PlaybackImageResultsDir is unimplemented")

  def PlaybackPerfDataDir(self):
    raise NotImplementedError("PlaybackPerfDataDir is unimplemented")

  def PlaybackPerfGraphsDir(self):
    raise NotImplementedError("PlaybackPerfGraphsDir is unimplemented")


class LocalSkpPlaybackDirs(SkpPlaybackDirs):
  """Encapsulates all required local dirs for skp playback BuildSteps."""

  def __init__(self, builder_name, perf_output_basedir):
    """Create an instance of LocalSkpPlaybackDirs."""
    SkpPlaybackDirs.__init__(self, builder_name, perf_output_basedir)

    self._local_playback_root_dir = os.path.abspath(
        os.path.join(os.pardir, ROOT_PLAYBACK_DIR_NAME))

  def PlaybackRootDir(self):
    """Returns the local playback root directory."""
    return self._local_playback_root_dir

  def PlaybackSkpDir(self):
    """Returns the local playback skp directory."""
    return os.path.join(
        self._local_playback_root_dir, SKPICTURES_DIR_NAME)

  def PlaybackImageResultsDir(self):
    """Returns the local playback image output directory."""
    return os.path.join(
        self._local_playback_root_dir, IMAGERESULTS_DIR_NAME,
        self._builder_name)

  def PlaybackPerfDataDir(self):
    """Returns the local playback perf data directory."""
    return os.path.abspath(os.path.join(
        self._perf_output_basedir, ROOT_PLAYBACK_DIR_NAME,
        self._builder_name, 'data')) if self._perf_output_basedir else None

  def PlaybackPerfGraphsDir(self):
    """Returns the local playback perf graphs directory."""
    return os.path.abspath(os.path.join(
        self._perf_output_basedir, ROOT_PLAYBACK_DIR_NAME,
        self._builder_name, 'graphs')) if self._perf_output_basedir else None


class StorageSkpPlaybackDirs(SkpPlaybackDirs):
  """Encapsulates all required storage dirs for skp playback BuildSteps."""

  def __init__(self, builder_name, perf_output_basedir):
    """Create an instance of StorageSkpPlaybackDirs."""
    SkpPlaybackDirs.__init__(self, builder_name, perf_output_basedir)

  def PlaybackRootDir(self):
    """Returns the relative storage playback root directory."""
    return ROOT_PLAYBACK_DIR_NAME

  def PlaybackSkpDir(self):
    """Returns the relative storage playback skp directory."""
    return posixpath.join(ROOT_PLAYBACK_DIR_NAME, SKPICTURES_DIR_NAME)

  def PlaybackImageResultsDir(self):
    """Returns the relative storage playback image output directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, IMAGERESULTS_DIR_NAME, self._builder_name)

  def PlaybackPerfDataDir(self):
    """Returns the relative playback perf data directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'perfdata', self._builder_name, 'data')

  def PlaybackPerfGraphsDir(self):
    """Returns the relative playback perf graphs directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'perfdata', self._builder_name, 'graphs')

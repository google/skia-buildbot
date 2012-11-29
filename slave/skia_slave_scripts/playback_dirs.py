# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Encapsulations all required directories for skp playback BuildSteps."""

import os
import posixpath


# The playback root directory name will be used both locally and on Google
# Storage.
ROOT_PLAYBACK_DIR_NAME = 'playback'
# The skpictures directory name will be used both locally and on Google Storage.
SKPICTURES_DIR_NAME = 'skps'


class SkpPlaybackDirs(object):
  """Interface for required directories for skp playback BuildSteps."""

  def __init__(self, builder_name, gm_image_subdir, perf_output_basedir):
    """Create an instance of SkpPlaybackDirs."""
    self._builder_name = builder_name
    self._gm_image_subdir = gm_image_subdir
    self._perf_output_basedir = perf_output_basedir

  def PlaybackRootDir(self):
    raise NotImplementedError("PlaybackRootDir is unimplemented!")

  def PlaybackSkpDir(self):
    raise NotImplementedError("PlaybackSkpDir is unimplemented!")

  def PlaybackGmActualDir(self):
    raise NotImplementedError("PlaybackGmActualDir is unimplemented")

  def PlaybackGmExpectedDir(self):
    raise NotImplementedError("PlaybackGmExpectedDir is unimplemented")

  def PlaybackPerfDataDir(self):
    raise NotImplementedError("PlaybackPerfDataDir is unimplemented")
  
  def PlaybackPerfGraphsDir(self):
    raise NotImplementedError("PlaybackPerfGraphsDir is unimplemented")


class LocalSkpPlaybackDirs(SkpPlaybackDirs):
  """Encapsulates all required local dirs for skp playback BuildSteps."""

  def __init__(self, builder_name, gm_image_subdir, perf_output_basedir):
    """Create an instance of LocalSkpPlaybackDirs."""
    SkpPlaybackDirs.__init__(self, builder_name, gm_image_subdir,
                             perf_output_basedir)

    self._local_playback_root_dir = os.path.abspath(
        os.path.join(os.pardir, ROOT_PLAYBACK_DIR_NAME))

  def PlaybackRootDir(self):
    """Returns the local playback root directory."""
    return self._local_playback_root_dir

  def PlaybackSkpDir(self):
    """Returns the local playback skp directory."""
    return os.path.join(
        self._local_playback_root_dir, SKPICTURES_DIR_NAME)

  def PlaybackGmActualDir(self):
    """Returns the local playback gm-actual directory."""
    return os.path.join(
        self._local_playback_root_dir, 'gm-actual',
        self._builder_name)

  def PlaybackGmExpectedDir(self):
    """Returns the local playback gm-expected directory."""
    return os.path.join(
        self._local_playback_root_dir, 'gm-expected', self._gm_image_subdir)

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

  def __init__(self, builder_name, gm_image_subdir, perf_output_basedir):
    """Create an instance of StorageSkpPlaybackDirs."""
    SkpPlaybackDirs.__init__(self, builder_name, gm_image_subdir,
                             perf_output_basedir)

  def PlaybackRootDir(self):
    """Returns the relative storage playback root directory."""
    return ROOT_PLAYBACK_DIR_NAME

  def PlaybackSkpDir(self):
    """Returns the relative storage playback skp directory."""
    return posixpath.join(ROOT_PLAYBACK_DIR_NAME, SKPICTURES_DIR_NAME)

  def PlaybackGmActualDir(self):
    """Returns the relative storage playback gm-actual directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'gm-actual', self._gm_image_subdir,
        self._builder_name, self._gm_image_subdir)

  def PlaybackGmExpectedDir(self):
    """Returns the relative storage playback gm-expected directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'gm-expected', self._gm_image_subdir)

  def PlaybackPerfDataDir(self):
    """Returns the relative playback perf data directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'perfdata', self._builder_name, 'data')

  def PlaybackPerfGraphsDir(self):
    """Returns the relative playback perf graphs directory."""
    return posixpath.join(
        ROOT_PLAYBACK_DIR_NAME, 'perfdata', self._builder_name, 'graphs')


#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's.

Based on gclient_factory.py and adds Skia-specific steps."""

from master.factory import gclient_factory

import skia_commands

import config

class SkiaFactory(gclient_factory.GClientFactory):
  """Encapsulates data and methods common to the Skia master.cfg files."""

  def __init__(self, build_subdir, target_platform=None, buildtype='Default',
               additional_gyp_args='', default_timeout=600,
               environment_variables=None, gm_image_subdir=None):
    """Instantiates a SkiaFactory as appropriate for this target_platform.

    build_subdir: string indicating path within slave directory
    target_platform: a string such as skia_commands.TARGET_PLATFORM_LINUX
    buildtype: 'Debug' or 'Release'
    additional_gyp_args: a string to append to the gyp command line
    default_timeout: default timeout for each command, in seconds
    environment_variables: dictionary of environment variables that should
        be passed to all commands
    gm_image_subdir: directory containing images for comparison against results
        of gm tool
    """
    # The only thing we use the BaseFactory for is to deal with gclient.
    gclient_solution = gclient_factory.GClientSolution(
        svn_url=config.Master.skia_url + 'trunk', name=build_subdir)
    gclient_factory.GClientFactory.__init__(
        self, build_dir='', solutions=[gclient_solution],
        target_platform=target_platform)
    self._additional_gyp_args = additional_gyp_args
    self._buildtype = buildtype
    self._factory = self.BaseFactory(factory_properties=None)
    self._gm_image_subdir = gm_image_subdir

    # Get an implementation of SkiaCommands as appropriate for
    # this target_platform.
    self._skia_cmd_obj = skia_commands.CreateSkiaCommands(
        target_platform=target_platform, factory=self._factory,
        target=buildtype, build_subdir=build_subdir, target_arch=None,
        default_timeout=default_timeout,
        environment_variables=environment_variables)

  def Build(self, clobber=False):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    if clobber:
      self._skia_cmd_obj.AddRun(
          run_command='rm -rf out', description='Clean')
    self._skia_cmd_obj.AddRun(
        run_command='./gyp_skia %s' % self._additional_gyp_args,
        description='Gyp')
    self._skia_cmd_obj.AddRun(
        run_command='make -C out core BUILDTYPE=%s' % self._buildtype,
        description='BuildCore')
    self._skia_cmd_obj.AddRun(
        run_command='make -C out tests BUILDTYPE=%s' % self._buildtype,
        description='BuildTests')
    self._skia_cmd_obj.AddRun(
        run_command='out/%s/tests' % self._buildtype, description='RunTests')
    self._skia_cmd_obj.AddRun(
        run_command='make -C out gm BUILDTYPE=%s' % self._buildtype,
        description='BuildGM')
    self._skia_cmd_obj.AddRun(
        run_command='out/%s/gm -r gm/%s' % (
            self._buildtype, self._gm_image_subdir),
        description='RunGM')
    self._skia_cmd_obj.AddRun(
        run_command='make -C out all BUILDTYPE=%s' % self._buildtype,
        description='BuildAllOtherTargets')
    return self._factory

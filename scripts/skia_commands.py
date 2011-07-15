#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Set of utilities to add commands to a buildbot factory.

This is based on commands.py and adds skia-specific commands."""

from buildbot.steps import shell

from master.factory import commands
import skia_factory

class SkiaCommands(commands.FactoryCommands):

  def __init__(self, factory, configuration, workdir, target_arch,
               default_timeout, target_platform, environment_variables):
    """Instantiates subclass of FactoryCommands appropriate for Skia.

    factory: a BaseFactory
    configuration: 'Debug' or 'Release'
    workdir: string indicating path within slave directory
    target_arch: string such as 'x64'
    default_timeout: default timeout for each command, in seconds
    target_platform: a string such as skia_factory.TARGET_PLATFORM_LINUX,
        to be passed into FactoryCommands.__init__()
    environment_variables: dictionary of environment variables that should
        be passed to all commands
    """
    commands.FactoryCommands.__init__(
        self, factory=factory, target=configuration,
        build_dir='', target_platform=target_platform)
    # Store some parameters that the subclass may want to use later.
    self.default_timeout = default_timeout
    self.environment_variables = environment_variables
    self.factory = factory
    self.target_arch = target_arch
    self.workdir = workdir

  def AddClean(self, build_target='clean', description='Clean', timeout=None):
    """Does a 'make clean'"""
    cmd = 'make %s' % build_target
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddBuild(self, build_target=None, description='Build', timeout=None):
    """Adds a compile step to the build."""
    if not build_target:
      raise ValueError, 'build_target not set'
    cmd = 'make %s' % build_target
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddRun(self, run_command=None, description='Run', timeout=None):
    """Runs something we built."""
    if not run_command:
      raise ValueError, 'run_command not set'
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=run_command,
                         workdir=self.workdir, env=self.environment_variables)

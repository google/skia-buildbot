# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Set of utilities to add commands to a buildbot factory.

This is based on commands.py and adds skia-specific commands."""

from buildbot.process.properties import WithProperties
from buildbot.steps import shell

from master.factory import commands

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
    # TODO(epoger): It would be better for this path to be specified by
    # an environment variable or some such, so that it is not dependent on the
    # path from CWD to slave/skia_slave_scripts... but for now, this will do.
    self._local_slave_script_dir = self.PathJoin(
        '..', '..', '..', '..', '..', '..', 'slave', 'skia_slave_scripts')

  def AddClean(self, build_target='clean', description='Clean', timeout=None):
    """Does a 'make clean'"""
    cmd = 'make %s' % build_target
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddBuild(self, build_target, description='Build', timeout=None):
    """Adds a compile step to the build."""
    cmd = 'make %s' % build_target
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddUploadToBucket(self, source_filepath,
                        dest_gsbase='gs://chromium-skia-gm',
                        description='Upload', timeout=None):
    """Adds a step that uploads a file to a Google Storage Bucket."""
    path_to_upload_script = self.PathJoin(
        self._local_slave_script_dir, 'upload_to_bucket.py')
    cmd = 'python %s --source_filepath=%s --dest_gsbase=%s' % (
        path_to_upload_script, source_filepath, dest_gsbase)
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddMergeIntoSvn(self, source_dir_path, dest_svn_url,
                      svn_username_file, svn_password_file,
                      commit_message=None, description='MergeIntoSvn',
                      timeout=None):
    """Adds a step that commits all files within a directory to a special
    SVN repository."""
    if not commit_message:
      commit_message = 'automated svn commit of %s step' % description

    path_to_upload_script = self.PathJoin(
        self._local_slave_script_dir, 'merge_into_svn.py')
    cmd = ['python', path_to_upload_script,
           '--source_dir_path', source_dir_path,
           '--dest_svn_url', dest_svn_url,
           '--svn_username_file', svn_username_file,
           '--svn_password_file', svn_password_file,
           '--commit_message', commit_message]
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=cmd, workdir=self.workdir,
                         env=self.environment_variables)

  def AddRunCommand(self, command, description='Run', timeout=None):
    """Runs an arbitrary command, perhaps a binary we built."""
    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(shell.ShellCommand, description=description,
                         timeout=timeout, command=command,
                         workdir=self.workdir, env=self.environment_variables)

  def AddRunCommandList(self, command_list, description='Run', timeout=None):
    """Runs a list of arbitrary commands."""
    # TODO(epoger): Change this so that build-step output shows each command
    # in the list separately--that will be a lot easier to follow.
    #
    # TODO(epoger): For now, this wraps the total command with WithProperties()
    # because *some* callers need it, and we can't use the string.join() command
    # to concatenate strings that have already been wrapped with
    # WithProperties().  Once I figure out how to make the build-step output
    # show each command separately, maybe I can remove this wrapper.
    self.AddRunCommand(command=WithProperties(' && '.join(command_list)),
                       description=description, timeout=timeout)

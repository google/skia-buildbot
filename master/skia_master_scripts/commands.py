# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Set of utilities to add commands to a buildbot factory.

This is based on commands.py and adds skia-specific commands.

TODO(borenet): Do we need this file at all?  Can't we just do everything
in factory.py?  (See https://codereview.chromium.org/248053003/ )
"""


from buildbot.process.properties import WithProperties
from master.factory import commands
import skia_build_step


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

  def AddMergeIntoSvn(self, source_dir_path, dest_svn_url, merge_dir_path,
                      svn_username_file, svn_password_file,
                      commit_message=None, description='MergeIntoSvn',
                      timeout=None, is_rebaseline_step=False):
    """Adds a step that commits all files within a directory to a special SVN
    repository."""
    if not commit_message:
      commit_message = 'automated svn commit of %s step' % description
    args = ['--commit_message', commit_message,
            '--dest_svn_url', dest_svn_url,
            '--merge_dir_path', merge_dir_path,
            '--source_dir_path', source_dir_path,
            '--svn_password_file', svn_password_file,
            '--svn_username_file', svn_username_file,
            ]
    self.AddSlaveScript(script=self.PathJoin('utils', 'merge_into_svn.py'),
                        args=args, description=description, timeout=timeout,
                        is_upload_step=True,
                        is_rebaseline_step=is_rebaseline_step)

  # TODO(borenet): Can kwargs be used to simplify this function declaration?
  def AddSlaveScript(self, script, args, description, timeout=None,
                     halt_on_failure=False, is_upload_step=False,
                     is_rebaseline_step=False, get_props_from_stdout=None,
                     workdir=None, do_step_if=None,
                     always_run=False, flunk_on_failure=True,
                     exception_on_failure=False):
    """Run a slave-side Python script as its own build step."""
    if workdir:
      path_to_script = script
      use_workdir = workdir
    else:
      path_to_script = self.PathJoin(self._local_slave_script_dir, script)
      use_workdir = self.workdir
    self.AddRunCommand(command=['python', path_to_script] + args,
                       description=description, timeout=timeout,
                       halt_on_failure=halt_on_failure,
                       is_upload_step=is_upload_step,
                       is_rebaseline_step=is_rebaseline_step,
                       get_props_from_stdout=get_props_from_stdout,
                       workdir=use_workdir,
                       do_step_if=do_step_if,
                       always_run=always_run,
                       flunk_on_failure=flunk_on_failure,
                       exception_on_failure=exception_on_failure)

  # TODO(borenet): Can kwargs be used to simplify this function declaration?
  def AddRunCommand(self, command, description='Run', timeout=None,
                    halt_on_failure=False, is_upload_step=False,
                    is_rebaseline_step=False, get_props_from_stdout=None,
                    workdir=None, do_step_if=None, always_run=False,
                    flunk_on_failure=True, exception_on_failure=False):
    """Runs an arbitrary command, perhaps a binary we built."""
    if description not in self.factory.dontskipsteps:
      if description in self.factory.skipsteps:
        return

    # If a developer has marked the step as dontskip, make sure it will run.
    if description in self.factory.dontskipsteps:
      do_step_if = True

    if not timeout:
      timeout = self.default_timeout
    self.factory.addStep(skia_build_step.SkiaBuildStep,
                         is_upload_step=is_upload_step,
                         is_rebaseline_step=is_rebaseline_step,
                         get_props_from_stdout=get_props_from_stdout,
                         description=description, timeout=timeout,
                         command=command, workdir=workdir or self.workdir,
                         env=self.environment_variables,
                         haltOnFailure=halt_on_failure,
                         doStepIf=do_step_if or skia_build_step.ShouldDoStep,
                         alwaysRun=always_run,
                         flunkOnFailure=flunk_on_failure,
                         exception_on_failure=exception_on_failure,
                         hideStepIf=lambda s: s.isSkipped())

  # TODO(borenet): Can kwargs be used to simplify this function declaration?
  def AddRunCommandList(self, command_list, description='Run', timeout=None,
                        halt_on_failure=False, is_upload_step=False,
                        is_rebaseline_step=False):
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
                       description=description, timeout=timeout,
                       halt_on_failure=halt_on_failure,
                       is_upload_step=is_upload_step,
                       is_rebaseline_step=is_rebaseline_step)

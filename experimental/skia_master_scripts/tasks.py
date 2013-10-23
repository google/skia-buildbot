# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Construction of a DAG of tasks for the buildbots to run. """


from buildbot.process import factory
from buildbot.scheduler import AnyBranchScheduler
from buildbot.steps import transfer, shell
from buildbot.steps import trigger
from buildbot.util import NotABranch

import graph_utils
import os
import posixpath
import skia_vars
import utils


BUILDBOT_SCRIPT_PATH = posixpath.join(os.pardir, os.pardir, os.pardir,
                                      os.pardir)


def _get_master_path(task_name, file_path):
  # TODO(borenet): The path MUST be organized by source stamp ID, or the
  # uploaded files can't be guaranteed to be valid.
  return posixpath.join('slave_files', 'ssid', task_name, file_path)


class Task(object):
  """Represents a work item for a buildbot."""

  _builder_prefix = 'b_%s'
  _factory_prefix = 'f_%s'
  _scheduler_prefix = 's_%s'

  def __init__(self, graph, name, cmd, workdir='build', slave_profile=None,
               requires_source_checkout=False):
    """Initialize the Task. This constructor is not intended to be used
    directly. Instead, use TaskManager to add Tasks.

    Args:
        graph: An instance of graph_utils.Graph to which this Task will be added
            as a Node.
        name: string; name of this Task.
        cmd: string or list of strings; the command line that this Task runs.
        workdir: string; working directory in which the command will run.
        slave_profile: dict outlining the requirements which a Buildslave must
            meet in order to perform this Task.
        requires_source_checkout: boolean indicating whether this Task requires
            an up-to-date source code checkout in order to run. If False, the
            Task does *not* download any code.
    """
    self._cmd = cmd
    self._files_to_download = []
    self._files_to_upload = set()
    self._graph = graph
    self._name = name
    self._requires_source_checkout = requires_source_checkout
    self._slave_profile = slave_profile or {}
    self._workdir = workdir
    self._id = self._graph.add_node(self)

  def add_dependency(self, task, download_files=None):
    """Add a Task to the set on which this Task depends.

    Args:
        task: Instance of Task which must run before this Task.
        download_files: List of paths to files to download from the Buildslave
            who runs the Task on which this Task depends.
    """
    self._graph.add_edge(self._id, task._id)
    if download_files:
      for file_path in download_files:
        self._files_to_download.append((task.name, file_path))
        task._files_to_upload.add(file_path)

  def get_build_factory(self):
    """Get the BuildFactory associated with this Task. Subclasses may override
    this method to produce different sets of BuildSteps.

    Returns:
        Instance of BuildFactory representing the Build to run for this Task.
    """
    f = factory.BuildFactory()

    # Always update the buildbot scripts.
    f.addStep(shell.ShellCommand(
        description='UpdateScripts',
        command='gclient sync',
        workdir=BUILDBOT_SCRIPT_PATH,
        haltOnFailure=True))

    # Sync code if this Task requires it.
    if self._requires_source_checkout:
      f.addStep(shell.ShellCommand(
          description='Update',
          command=('gclient config https://skia.googlesource.com/skia.git; '
                   'gclient sync'),
          workdir=posixpath.join(self.workdir, os.pardir),
          haltOnFailure=True))

    # Download any required files from the master.
    for dependency_name, file_path in self._files_to_download:
      f.addStep(transfer.FileDownload(
          mastersrc=_get_master_path(dependency_name, file_path),
          slavedest=file_path,
          workdir=self.workdir,
          mode=0755,
          haltOnFailure=True
      ))

    # Run the command required of this step.
    f.addStep(shell.ShellCommand(
        description=self.name,
        command=self._cmd,
        workdir=self.workdir))

    # Upload any required files to the master.
    for file_to_upload in self._files_to_upload:
      f.addStep(transfer.FileUpload(
          slavesrc=file_to_upload,
          masterdest=_get_master_path(self.name, file_to_upload),
          workdir=self.workdir,
          mode=0755,
      ))

    return f

  def can_be_performed_by(self, buildslave):
    """Determine whether the given Buildslave can perform this Task.

    This function compares the profile dict of the Task with the profile dict of
    the Buildslave. The Buildslave may run the Task if the Buildslave's profile
    is a superset of this Task's profile.

    Args:
        buildslave: dictionary describing a Buildslave.

    Returns:
        True if the Buildslave may run this Task and false otherwise.
    """
    if self._slave_profile and not buildslave.get('profile'):
      return False
    for property_name, desired_value in self._slave_profile.iteritems():
      if (not buildslave['profile'].get(property_name) or
          buildslave['profile'][property_name] != desired_value):
        return False
    return True

  @property
  def name(self):
    """The name of this Task."""
    return self._name

  @property
  def workdir(self):
    """Working directory where this Task should run."""
    return self._workdir

  @property
  def slave_profile(self):
    return self._slave_profile

  @property
  def dependencies(self):
    """List of Tasks on which this Task depends."""
    return [self._graph[child_id]
            for child_id in self._graph.children(self._id)]

  @property
  def builder_name(self):
    """Name of the builder associated with this Task."""
    return Task._builder_prefix % self.name

  @property
  def factory_name(self):
    """Name of the BuildFactory associated with this Task."""
    return Task._factory_prefix % self.name

  @property
  def scheduler_name(self):
    """Name of the Scheduler associated with this Task."""
    return Task._scheduler_prefix % self.name


class TaskManager(graph_utils.Graph):
  """Manages a set of Tasks."""

  def add_task(self, **kwargs):
    """Add a new task to the Graph.

    Returns:
        A new Task instance.
    """
    return Task(self, **kwargs)

  def create_builders_from_dag(self, slaves, config):
    """Given a Directed Acyclic Graph whose nodes are Tasks and whose edges are
    dependencies between tasks, sets up Schedulers, Builders, and BuildFactorys
    which represent the same dependency relationships, and assigns Builders to
    appropriate Buildslaves according to their profile.

    Args:
        slaves: List of Buildslave configuration dictionaries.
        config: Configuration dictionary for the Buildbot master.
    """
    helper = utils.Helper()

    # Perform a topological sort of the graph so that we can set up the
    # dependencies more easily.
    sorted_tasks = self.topological_sort()
    # Create a Scheduler, BuildFactory, and Builder for each Task.
    for task_id in reversed(sorted_tasks):
      task = self[task_id]

      # Create a Scheduler.
      scheduler_name = task.scheduler_name
      helper.Dependent(scheduler_name, [dep.scheduler_name
                                        for dep in task.dependencies])

      # Create a BuildFactory.
      factory_name = task.factory_name
      helper.Factory(factory_name, task.get_build_factory())

      # Create a Builder.
      builder_name = task.builder_name
      helper.Builder(name=builder_name,
                     factory=factory_name,
                     scheduler=scheduler_name,
                     auto_reboot=False)

      # Add the Builder to the appropriate Buildslaves.
      for buildslave in slaves:
        if not buildslave.get('builder'):
          buildslave['builder'] = []
        if task.can_be_performed_by(buildslave):
          buildslave['builder'].append(builder_name)

    # Remove any unused Buildslaves to satisfy the configuration test.
    for buildslave in slaves:
      if not buildslave.get('builder'):
        slaves.remove(buildslave)

    helper.Update(config)

# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Construction of a DAG of tasks for the buildbots to run. """


from buildbot.process import factory
from buildbot.scheduler import AnyBranchScheduler
from buildbot.steps import shell
from buildbot.steps import trigger
from buildbot.util import NotABranch

import graph_utils
import skia_vars
import utils


class TaskManager(graph_utils.Graph):
  """Manages a set of Tasks."""

  def add_task(self, **kwargs):
    """Add a new task to the Graph.

    Returns:
        A new Task instance.
    """
    return Task(self, **kwargs)


class Task(object):
  """Represents a work item for a buildbot."""

  _builder_prefix = 'b_%s'
  _factory_prefix = 'f_%s'
  _scheduler_prefix = 's_%s'

  def __init__(self, graph, name, cmd, workdir='build'):
    self._graph = graph
    self._name = name
    self._cmd = cmd
    self._workdir = workdir
    self._id = self._graph.add_node(self)

  def add_dependency(self, task):
    """Add a Task to the set on which this Task depends.

    Args:
        task: Instance of Task which must run before this Task.
    """
    self._graph.add_edge(self._id, task._id)

  def get_build_step(self):
    """Get the BuildStep associated with this Task. Subclasses may override this
    method to produce different types of BuildSteps.

    Returns:
        Instance of a subclass of BuildStep to run.
    """
    return shell.ShellCommand(description=self.name,
                              command=self._cmd,
                              workdir=self.workdir)

  @property
  def name(self):
    """The name of this Task."""
    return self._name

  @property
  def workdir(self):
    """Working directory where this Task should run."""
    return self._workdir

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


def create_builders_from_dag(task_mgr, config):
  """Given a Directed Acyclic Graph whose nodes are Tasks and whose edges are
  dependencies between tasks, sets up Schedulers, Builders, and BuildFactorys
  which represent the same dependency relationships.

  Args:
      task_mgr: Instance of TaskManager.
      config: Configuration dictionary for the Buildbot master.
  """
  if not isinstance(task_mgr, TaskManager):
    raise ValueError('task_mgr must be an instance of TaskManager.')

  helper = utils.Helper()

  # Perform a topological sort of the graph so that we can set up the
  # dependencies more easily.
  sorted_tasks = task_mgr.topological_sort()

  # Create a Scheduler, BuildFactory, and Builder for each Task.
  for task_id in reversed(sorted_tasks):
    task = task_mgr[task_id]
    print '%s: %s' % (task.name, task.dependencies)

    # Create a Scheduler.
    scheduler_name = task.scheduler_name
    helper.Dependent(scheduler_name, [dep.scheduler_name
                                      for dep in task.dependencies])

    # Create a BuildFactory.
    f = factory.BuildFactory()
    f.addStep(task.get_build_step())
    factory_name = task.factory_name
    helper.Factory(factory_name, f)

    # Create a Builder.
    builder_name = task.builder_name
    helper.Builder(name=builder_name,
                   factory=factory_name,
                   scheduler=scheduler_name,
                   auto_reboot=False)

  helper.Update(config)

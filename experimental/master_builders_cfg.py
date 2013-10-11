# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Sets up all the builders we want this buildbot master to run."""


from skia_master_scripts import tasks
from skia_master_scripts import graph_utils
from skia_master_scripts import utils


def Update(config, active_master, cfg):
  """Set up the builders for this build master, using a directed acyclic graph
  of Tasks and their dependencies.

  Modifies the passed-in cfg dict according to the constructed set of Tasks.

  Args:
      config: Module containing configuration information for the build master.
      active_master: Object representing the active build master.
      cfg: Configuration dictionary for the build master.
  """
  task_mgr = tasks.TaskManager()

  sync = task_mgr.add_task(name='Update', cmd=['sleep', '10'])

  compile = task_mgr.add_task(name='Build', cmd=['sleep', '10'],
                              workdir='build/skia')
  compile.add_dependency(sync)

  run_tests = task_mgr.add_task(name='RunTests', cmd=['sleep', '10'],
                                workdir='build/skia')
  run_tests.add_dependency(compile)

  run_gm = task_mgr.add_task(name='RunGM', cmd=['sleep', '10'],
                             workdir='build/skia')
  run_gm.add_dependency(compile)

  run_bench = task_mgr.add_task(name='RunBench', cmd=['sleep', '10'],
                                workdir='build/skia')
  run_bench.add_dependency(compile)

  all_done = task_mgr.add_task(name='AllFinished', cmd=['sleep', '10'],
                               workdir='build/skia')
  all_done.add_dependency(run_tests)
  all_done.add_dependency(run_gm)
  all_done.add_dependency(run_bench)

  return tasks.create_builders_from_dag(task_mgr, cfg)

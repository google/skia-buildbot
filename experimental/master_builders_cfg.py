# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Sets up all the builders we want this buildbot master to run."""


from skia_master_scripts import tasks
from skia_master_scripts import graph_utils
from skia_master_scripts import utils


def Update(config, active_master, slaves, cfg):
  """Set up the builders for this build master, using a directed acyclic graph
  of Tasks and their dependencies.

  Modifies the passed-in cfg dict according to the constructed set of Tasks.

  Args:
      config: Module containing configuration information for the build master.
      active_master: Object representing the active build master.
      slaves: List of Buildslave dictionaries.
      cfg: Configuration dictionary for the build master.
  """
  task_mgr = tasks.TaskManager()

  # Some dummy commands to run which sleep and then succeed or  fail. Sleeping
  # is helpful so that we can see the buildsets fire in real time.
  succeed = 'sleep 5; exit 0'
  fail = 'sleep 5; exit 1'

  # These profiles describe the type of buildslave which may run certain tasks.
  # Pre-build a few so that we can re-use them below.
  slave_ubuntu12 = {
    'os': 'Ubuntu12',
  }
  slave_ubuntu12_ati5770 = dict(gpu='ATI5770', **slave_ubuntu12)

  # Set up some tasks.
  compile = task_mgr.add_task(name='Build', cmd=succeed,
                              workdir='build/skia',
                              slave_profile=slave_ubuntu12,
                              requires_source_checkout=True)

  run_tests = task_mgr.add_task(name='RunTests', cmd=succeed,
                                workdir='build/skia',
                                slave_profile=slave_ubuntu12)
  run_tests.add_dependency(compile, download_file='out/Debug/tests')

  run_gm = task_mgr.add_task(name='RunGM', cmd=succeed,
                             workdir='build/skia',
                             slave_profile=slave_ubuntu12)
  run_gm.add_dependency(compile, download_file='out/Debug/gm')

  run_gm_gpu = task_mgr.add_task(name='RunGM_ATI5770', cmd=succeed,
                                 workdir='build/skia',
                                 slave_profile=slave_ubuntu12_ati5770)
  run_gm_gpu.add_dependency(compile, download_file='out/Debug/gm')

  run_bench = task_mgr.add_task(name='RunBench', cmd=succeed,
                                workdir='build/skia',
                                slave_profile=slave_ubuntu12)
  run_bench.add_dependency(compile, download_file='out/Debug/bench')

  all_done = task_mgr.add_task(name='AllFinished', cmd=succeed,
                               workdir='build/skia',
                               slave_profile=slave_ubuntu12)
  all_done.add_dependency(run_tests)
  all_done.add_dependency(run_gm)
  all_done.add_dependency(run_gm_gpu)
  all_done.add_dependency(run_bench)

  return task_mgr.create_builders_from_dag(slaves, cfg)

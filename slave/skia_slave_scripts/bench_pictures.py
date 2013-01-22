#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from utils import shell_utils
from build_step import BuildStep
from run_bench import BenchArgs
from run_bench import RunBench
from run_bench import PreBench
import os
import sys


# Skipping these for now to avoid excessively long cycle times.
RUNNING_ALL_CONFIGURATIONS = False


class BenchPictures(RunBench):
  def __init__(self, timeout=16800, no_output_timeout=16800, **kwargs):
    super(BenchPictures, self).__init__(timeout=timeout,
                                        no_output_timeout=no_output_timeout,
                                        **kwargs)

  def _BuildDataFile(self, perf_dir, args):
    data_file = '%s_skp_%s' % (
        super(BenchPictures, self)._BuildDataFile(perf_dir),
        '_'.join(args).replace('-', '').replace(':', '-'))
    return data_file

  def _GetSkpDir(self):
    return self._skp_dir

  def _GetPerfDataDir(self):
    return self._perf_data_dir

  def _PopulateSkpDir(self):
    # The skp dir comes from skia repository, nothing to do here.
    pass

  def _DoBenchPictures(self, args):
    cmd = [self._PathToBinary('bench_pictures'), self._GetSkpDir()] + args
    if self._GetPerfDataDir():
      PreBench(self._GetPerfDataDir())
      cmd += BenchArgs(repeats=self.BENCH_REPEAT_COUNT,
                       data_file=self._BuildDataFile(self._GetPerfDataDir(),
                                                     args))
    shell_utils.Bash(cmd)

  def _Run(self):
    self._PopulateSkpDir()

    # Determine which configs to run
    if self._configuration == 'Debug':
      cfg_name = 'debug'
    else:
      cfg_name = self._args['bench_pictures_cfg']

    vars = {'import_path': 'tools'}
    execfile(os.path.join('tools', 'bench_pictures.cfg'), vars)
    bench_pictures_cfg = vars['bench_pictures_cfg']
    if bench_pictures_cfg.has_key(cfg_name):
      my_configs = bench_pictures_cfg[cfg_name]
    else:
      my_configs = bench_pictures_cfg['default']
      print 'Warning: no bench_pictures_cfg found for %s! ' \
            'Using default.' % cfg_name

    # Run each config
    errors = []
    for config in my_configs:
      args = []
      for key, value in config.iteritems():
        args.append('--' + key)
        if value is True:
          # The flag doesn't take the form "--key value", just "--key"
          continue
        if type(value).__name__ == 'list':
          args.extend(value)
        else:
          args.append(value)
      try:
        self._DoBenchPictures(args)
      except Exception as e:
        print e
        errors.append(e)
    if errors:
      raise errors[0]


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchPictures))

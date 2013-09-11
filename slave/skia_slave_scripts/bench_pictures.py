#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench_pictures executable. """

from build_step import BuildStep
from run_bench import BenchArgs, GetSvnRevision
import os
import sys


BENCH_REPEAT_COUNT = 20


class BenchPictures(BuildStep):
  def __init__(self, timeout=16800, no_output_timeout=16800, **kwargs):
    super(BenchPictures, self).__init__(timeout=timeout,
                                        no_output_timeout=no_output_timeout,
                                        **kwargs)

  # pylint: disable=W0221
  def _BuildDataFile(self, args):
    filename = '_'.join(['bench', 'r%s' % GetSvnRevision(self._got_revision),
                         'data', 'skp'] + args)
    full_path = os.path.join(self._device_dirs.PerfDir(),
        filename.replace('-', '').replace(':', '-'))
    return full_path

  def _DoBenchPictures(self, args):
    arguments = ['-r', self._device_dirs.SKPDir()] + args
    if self._perf_data_dir:
      arguments.extend(BenchArgs(data_file=self._BuildDataFile(args)))
      # For bench_pictures we use the --repeat and --logPerIter flags so that we
      # can compensate for noisy performance.
      arguments.extend(['--repeat', str(BENCH_REPEAT_COUNT), '--logPerIter'])
    self._flavor_utils.RunFlavoredCmd('bench_pictures', arguments)

  def _Run(self):
    # Determine which configs to run
    if self._configuration == 'Debug':
      cfg_name = 'debug'
    else:
      cfg_name = self._args['bench_pictures_cfg']

    config_vars = {'import_path': 'tools'}
    execfile(os.path.join('tools', 'bench_pictures.cfg'), config_vars)
    bench_pictures_cfg = config_vars['bench_pictures_cfg']
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

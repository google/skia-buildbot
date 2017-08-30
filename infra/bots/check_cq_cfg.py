#!/usr/bin/env python
# Copyright (c) 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


import json
import os
import sys

INFRABOTS_DIR = os.path.abspath(os.path.dirname(__file__))
SKIA_DIR = os.path.abspath(os.path.join(INFRABOTS_DIR, os.pardir, os.pardir))
DEPOT_TOOLS_DIR = os.path.join(INFRABOTS_DIR, '.recipe_deps', 'depot_tools')
if os.environ.get('CHROME_HEADLESS') == '1' and os.environ.get('DEPOT_TOOLS'):
  DEPOT_TOOLS_DIR = os.environ['DEPOT_TOOLS']
sys.path.append(os.path.join(DEPOT_TOOLS_DIR, 'third_party', 'cq_client', 'v1'))
sys.path.append(os.path.join(DEPOT_TOOLS_DIR, 'third_party'))

import protobuf26
import cq_pb2


def main():
  # Load the cq.cfg file.
  cq_cfg_file = os.path.join(SKIA_DIR, 'infra', 'branch-config', 'cq.cfg')
  with open(cq_cfg_file, 'rb') as f:
    contents = f.read()
  cfg = cq_pb2.Config()
  protobuf26.text_format.Merge(contents, cfg)

  # Load the tasks.json file.
  tasks_json_file = os.path.join(INFRABOTS_DIR, 'tasks.json')
  with open(tasks_json_file, 'rb') as f:
    tasks_json = json.load(f)
  jobs = tasks_json['jobs']

  # Ensure that everything in cq.cfg is in tasks.json.
  missing = []
  for b in cfg.verifiers.try_job.buckets:
    if b.name.startswith('skia'):
      for builder in b.builders:
        if not jobs.get(builder.name):
          missing.append(builder.name)
  if missing:
    cq_cfg_relpath = os.path.relpath(cq_cfg_file, SKIA_DIR)
    tasks_json_relpath = os.path.relpath(tasks_json_file, SKIA_DIR)
    print >> sys.stderr, ('%s file has builders which do not exist as jobs in '
                          '%s:\n  %s' % (cq_cfg_relpath, tasks_json_relpath,
                                         '\n  '.join(missing)))
    sys.exit(1)
  print 'OK'


if __name__ == '__main__':
  main()

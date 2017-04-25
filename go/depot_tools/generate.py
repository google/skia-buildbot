#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Create gen_version.go with the depot_tools rev in infra/config/recipes.cfg"""


import json
import os
import subprocess
import sys


def generate_version_file(source_file, output_file):
  recipes_cfg = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                             os.pardir, os.pardir, 'infra', 'config',
                             'recipes.cfg')
  with open(recipes_cfg, 'r') as f:
    recipe_cfg_json = json.load(f)
  depot_tools_version = None
  for dep in recipe_cfg_json['deps']:
    if dep['project_id'] == 'depot_tools':
      depot_tools_version = dep['revision']
  if not depot_tools_version:
    raise Exception('No depot_tools version found!')

  version_info = {
    'depot_tools_version': depot_tools_version,
  }
  with open(output_file, 'w') as o:
    with open(source_file) as i:
      o.write(i.read() % version_info)


if __name__ == '__main__':
  generate_version_file(*sys.argv[1:])

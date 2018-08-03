#!/usr/bin/env python
# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import sys

import merge_api
import results_merger


def StandardIsolatedScriptMerge(output_json, jsons_to_merge):
  """Merge the contents of one or more results JSONs into a single JSON.

  Args:
    output_json: A path to a JSON file to which the merged results should be
      written.
    jsons_to_merge: A list of paths to JSON files that should be merged.
  """
  shard_results_list = []
  for j in jsons_to_merge:
    with open(j) as f:
      try:
        json_contents = json.load(f)
      except ValueError:
        raise ValueError('Failed to parse JSON from %s' % j)
      shard_results_list.append(json_contents)
  merged_results = results_merger.merge_test_results(shard_results_list)

  with open(output_json, 'w') as f:
    json.dump(merged_results, f)

  return 0


def main(raw_args):
  parser = merge_api.ArgumentParser()
  args = parser.parse_args(raw_args)
  return StandardIsolatedScriptMerge(args.output_json, args.jsons_to_merge)


if __name__ == '__main__':
  sys.exit(main(sys.argv[1:]))

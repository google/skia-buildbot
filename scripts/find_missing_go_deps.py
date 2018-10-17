#!/usr/bin/env python
#
# Copyright 2018 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


import argparse
import json
import subprocess


def get_missing_deps(pkg):
  output = subprocess.check_output(['go', 'list', '--json', pkg])
  # When specifying multiple packages, unfortunately "go list" does not return
  # valid JSON. Instead, each package is valid JSON and simply separated by
  # newlines. Massage the output into valid JSON.
  lines = ['[']
  split_output = output.splitlines()
  for idx, line in enumerate(split_output):
    if line == '}' and idx != len(split_output)-1:
      line += ','
    lines.append(line)
  lines.append(']')
  info = json.loads('\n'.join(lines))
  rv = {}
  for pkg in info:
    if pkg.get('DepsErrors'):
      for err in pkg['DepsErrors']:
        if not err.get('IsImportCycle') and err.get('ImportStack'):
          rv[err['ImportStack'][-1]] = True
  return rv.keys()


def main():
  parser = argparse.ArgumentParser()
  parser.add_argument(
      '--json', help='Write output in JSON format.', action='store_true')
  parser.add_argument(
      '--output',
      help='If provided write output to this file instead of stdout.')
  args = parser.parse_args()

  missing = get_missing_deps('go.skia.org/infra/...')
  output = ''
  if missing:
    if args.json:
      output = json.dumps(missing, indent=2, sort_keys=True)
    else:
      output = '\n'.join(missing)
  if args.output:
    with open(args.output, 'w') as f:
      f.write(output)
  else:
    print output


if __name__ == '__main__':
  main()

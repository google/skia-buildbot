#!/usr/bin/env python


import json
import pprint
import subprocess
import sys


def get_imports(pkg):
  output = json.loads(subprocess.check_output(['go', 'list', '--json', pkg]))
  builtin = []
  other = []
  for i in output.get('Imports', []):
    if '.' in i.split('/')[0]:
      other.append(i)
    else:
      builtin.append(i)
  return builtin, other


def find_import_path(start_pkg, find_pkg):
  cache = {}

  def find_import_path_helper(start_pkg, find_pkg):
    if cache.get(start_pkg) is not None:
      return cache[start_pkg]
    sys.stdout.write('.')
    sys.stdout.flush()
    builtin, other = get_imports(start_pkg)
    found = {}
    for pkg in builtin:
      if find_pkg == pkg:
        found[start_pkg] = find_pkg
    for pkg in other:
      if find_pkg == pkg:
        found[start_pkg] = find_pkg
    for pkg in other:
      for k, v in find_import_path_helper(pkg, find_pkg).iteritems():
        found[k] = v
    rv = {}
    if found:
      rv[start_pkg] = found
    cache[start_pkg] = rv
    return rv

  rv = find_import_path_helper(start_pkg, find_pkg)
  sys.stdout.write('\n')
  sys.stdout.flush()
  return rv


def main(args):
  if len(args) != 2:
    raise Exception('Usage: find_importer.py <start> <pkg>')
  start_pkg = args[0]
  find_pkg = args[1]
  result = find_import_path(start_pkg, find_pkg)
  print json.dumps(result, indent=2, sort_keys=True)


if __name__ == '__main__':
  main(sys.argv[1:])

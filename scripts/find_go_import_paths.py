#!/usr/bin/env python
# Copyright (c) 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Find all paths by which the given package imports another.

This is useful for tracking down and removing imports of a given package, or
tracking usage.
"""


import json
import pprint
import subprocess
import sys


def get_imports(pkg):
  """Return built-in and not-built-in packages imported by the given package."""
  output = json.loads(subprocess.check_output(['go', 'list', '--json', pkg]))
  builtin = []
  other = []
  for i in output.get('Imports', []):
    # This is a little hacky, but it works in practice: builtin packages do not
    # have a dot in their first path component, while others do.
    if '.' in i.split('/')[0]:
      other.append(i)
    else:
      builtin.append(i)
  return builtin, other


def find_import_path(start_pkg, find_pkg):
  """Return a map showing the import path(s) from one package to the other."""
  cache = {}

  def find_import_path_helper(current_pkg):
    """Helper which uses a cache to avoid searching the same package again."""
    # If we have a cached result, return it.
    if cache.get(current_pkg):
      return cache[current_pkg]

    # Find the imports for the current package.
    builtin, other = get_imports(current_pkg)

    # Search for the requested package.
    found = {}

    # Does the current package import the requested package?
    for pkg in builtin:
      if find_pkg == pkg:
        found[find_pkg] = True
    for pkg in other:
      if find_pkg == pkg:
        found[find_pkg] = True

    # Recursively search each package imported by the current package, excluding
    # built-in packages.
    for pkg in other:
      for k, v in find_import_path_helper(pkg).iteritems():
        found[k] = v

    # If we found any import paths, cache and return them.
    rv = {}
    if found:
      rv[current_pkg] = found
    cache[current_pkg] = rv

    # This takes a while; show something to the user to indicate that we're
    # actually making progress.
    sys.stdout.write('.')
    sys.stdout.flush()

    return rv

  # Call into the helper function.
  rv = find_import_path_helper(start_pkg)

  # Don't leave stdout hanging on the same line.
  sys.stdout.write('\n')
  sys.stdout.flush()
  return rv


def main(args):
  if len(args) != 2:
    raise Exception('Usage: find_importer.py <start_pkg> <find_pkg>')
  start_pkg = args[0]
  find_pkg = args[1]
  resultMap = find_import_path(start_pkg, find_pkg)

  # Print the results.
  results = []
  def collect(current, prefix):
    for k, v in current.iteritems():
      if v is True:
        results.append(prefix + ' -> %s' % find_pkg)
      else:
        new_prefix = k
        if prefix:
          new_prefix = prefix + ' -> %s' % k
        collect(v, new_prefix)
  collect(resultMap, '')

  print 'Found import paths:'
  for line in results:
    print '\t%s' % line


if __name__ == '__main__':
  main(sys.argv[1:])

#!/usr/bin/env python
#
# Copyright 2021 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Check for Python scripts which are incompatible with Python 3."""


import ast
import os
import subprocess
import sys


def check_file(fp):
  content = open(fp, 'r').read()
  try:
    parsed = ast.parse(content)
    if not parsed:
      return False
    return True
  except SyntaxError:
    return False


def check_repo(path):
  files = subprocess.check_output(['git', 'ls-files'], cwd=path).splitlines()
  incompatible = []
  for f in files:
    f = f.decode(sys.stdout.encoding)
    if f.endswith('.py'):
      if not check_file(os.path.join(path, f)):
        incompatible.append(f)
  return incompatible


def __main__(argv):
  if len(argv) != 2:
    print('Usage: %s <repo path>' % __file__)
    sys.exit(1)
  incompatible = check_repo(argv[1])
  if len(incompatible) > 0:
    print('Incompatible Python scripts:')
    for f in incompatible:
      print(f)
    sys.exit(1)


if __name__ == '__main__':
  __main__(sys.argv)

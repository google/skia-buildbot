#!/usr/bin/env python3


import ast
import os
import shutil
import subprocess
import sys


def load_isolate(isolate):
  base = os.path.dirname(os.path.abspath(isolate))
  with open(isolate) as f:
    content = ast.literal_eval(f.read())
  files = set()
  for f in content.get('variables', {}).get('files', []):
    files.add(os.path.normpath(os.path.join(base, f)))
  for inc in content.get('includes', []):
    for f in load_isolate(os.path.join(base, inc)):
      files.add(f)
  return sorted(files)


def expand(f):
  if os.path.isfile(f):
    return [f]
  elif os.path.isdir(f):
    return expand_all([os.path.join(f, child) for child in os.listdir(f)])
  else:
    print('%s is not a file or dir; ignoring' % f)
    return []


def expand_all(files):
  rv = []
  for f in files:
    rv.extend(expand(f))
  return rv


def copy(files, root, dest):
  for f in files:
    rel = os.path.relpath(f, root)
    d = os.path.join(dest, rel)
    dirname, filename = os.path.split(d)
    subprocess.check_call(['mkdir', '-p', dirname])
    shutil.copy(f, d)


def clear_timestamp(f):
  subprocess.check_call(['touch', '-a', '-c', '-m', '-t', '197001010000', f])

def clear_timestamps(dest):
  for root, dirs, files in os.walk(dest):
    for d in dirs:
      clear_timestamp(d)
    for f in files:
      clear_timestamp(f)


def main():
  if len(sys.argv) != 4:
    print('USAGE: %s <root> <isolate filename> <dest>' % sys.argv[0])
    sys.exit(1)

  root = sys.argv[1]
  isolate = sys.argv[2]
  dest = sys.argv[3]

  files = load_isolate(isolate)
  all_files = expand_all(files)
  copy(all_files, root, dest)
  clear_timestamps(dest)


if __name__ == '__main__':
  main()

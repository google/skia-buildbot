#!/usr/bin/env python

import argparse
import os
import sys

def main():
  parser = argparse.ArgumentParser(
      description='Upload screenshots produced by Puppeteer tests to Gold.')
  parser.add_argument(
      '--issue', metavar='CL_ID', type=int, default=None, help='changelist ID')
  parser.add_argument(
      '--patch_set', metavar='PS_NUM', type=int, default=None, help='patch_set number')
  parser.add_argument(
      '--revision', metavar='HASH', type=str, default=None, help='git commit hash')
  parser.add_argument(
      '--task_id', metavar='TASK_ID', type=str, default=None, help='tryjob ID')

  args = parser.parse_args()

  # Either none or all flags should be set.
  arg_values = [args.issue, args.patch_set, args.revision, args.task_id]
  if not (all(v is None for v in arg_values) or
          all(v is not None for v in arg_values)):
    sys.stderr.write(
        'Please provide either none or all flags --issue, --patch_set, ' +
        '--revision and --task_id.\n')
    sys.exit(1)

  if args.issue is not None and args.issue < 1:
    sys.stderr.write('Flag --issue must be a positive integer.\n')
    sys.exit(1)
  if args.patch_set is not None and args.patch_set < 1:
    sys.stderr.write('Flag --patch_set must be a positive integer.\n')
    sys.exit(1)

  # TODO(lovisolo): Implement.
  print('[NOT IMPLEMENTED] Uploading screenshots produced by Puppeteer tests to Gold...')
  print('  issue: %d' % args.issue if args.issue else '  no issue')
  print('  patch_set: %d' % args.patch_set if args.patch_set else '  no patch_set')
  print('  revision: %s' % args.revision if args.revision else '  no revision')
  print('  task_id: %s' % args.task_id if args.task_id else '  no task_id')
  print('')
  print('$ pwd')
  os.system('pwd')
  print('')
  print('$ ls -l')
  os.system('ls -l')

if __name__ == '__main__':
  main()

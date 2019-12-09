#!/usr/bin/env python

import argparse
import os
import sys

def main():
  parser = argparse.ArgumentParser(
      description='Upload screenshots produced by Puppeteer tests to Gold.')
  parser.add_argument(
      '--issue', metavar='ISSUE_ID', type=int, default=None, help='changelist ID')
  parser.add_argument(
      '--patchset', metavar='PATCHSET_NUM', type=int, default=None, help='patchset number')
  parser.add_argument(
      '--commit_hash', metavar='HASH', type=str, default=None, help='git commit hash')
  parser.add_argument(
      '--tryjob_id', metavar='TRYJOB_ID', type=str, default=None, help='tryjob ID')

  args = parser.parse_args()

  # Either none or all flags should be set.
  arg_values = [args.issue, args.patchset, args.commit_hash, args.tryjob_id]
  if not (all(v is None for v in arg_values) or
          all(v is not None for v in arg_values)):
    sys.stderr.write(
        'Please provide either none or all flags --issue, --patchset, ' +
        '--commit_hash and --tryjob_id.\n')
    sys.exit(1)

  if args.issue is not None and args.issue < 1:
    sys.stderr.write('Flag --issue must be a positive integer.\n')
    sys.exit(1)
  if args.patchset is not None and args.patchset < 1:
    sys.stderr.write('Flag --patchset must be a positive integer.\n')
    sys.exit(1)

  # TODO(lovisolo): Implement.
  print('[NOT IMPLEMENTED] Uploading screenshots produced by Puppeteer tests to Gold...')
  print('  issue: %d' % args.issue if args.issue else '  no issue')
  print('  patchset: %d' % args.patchset if args.patchset else '  no patchset')
  print('  commit_hash: %s' % args.commit_hash if args.commit_hash else '  no commit_hash')
  print('  tryjob_id: %s' % args.tryjob_id if args.tryjob_id else '  no tryjob_id')
  print('')
  print('$ pwd')
  os.system('pwd')
  print('')
  print('$ ls -l')
  os.system('ls -l')

if __name__ == '__main__':
  main()

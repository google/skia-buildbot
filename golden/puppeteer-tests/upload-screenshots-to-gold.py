#!/usr/bin/env python

import argparse
import sys

def main():
  parser = argparse.ArgumentParser(
      description='Upload screenshots produced by Puppeteer tests to Gold.')
  parser.add_argument(
      '--issue', metavar='ID', type=int, default=None, help='changelist ID')
  parser.add_argument(
      '--patch_set', metavar='PS', type=int, default=None, help='patchset number')
  args = parser.parse_args()

  if (args.issue is None) != (args.patch_set is None):
    sys.stderr.write('Please set either none or both flags --issue and --patch_set.\n')
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

if __name__ == '__main__':
  main()

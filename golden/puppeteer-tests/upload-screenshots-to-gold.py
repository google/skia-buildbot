#!/usr/bin/env python

import argparse
import os
import shutil
import sys
import tempfile


def main():
  parser = argparse.ArgumentParser(
      description='Upload screenshots produced by Puppeteer tests to Gold.')
  parser.add_argument(
      '--path_to_goldctl', metavar='PATH_TO_GOLDCTL', type=str, required=True,
      help='path to the goldctl binary')
  parser.add_argument(
      '--revision', metavar='HASH', type=str, required=True,
      help='git commit hash')
  parser.add_argument(
      '--issue', metavar='CL_ID', type=int, default=None, help='changelist ID')
  parser.add_argument(
      '--patch_set', metavar='PS_NUM', type=int, default=None,
      help='patch set number')
  parser.add_argument(
      '--task_id', metavar='TASK_ID', type=str, default=None, help='tryjob ID')
  parser.add_argument(
      '--local', type=bool, nargs='?', const=True, default=False,
      help='authorize goldctl using gsutil (for local development only)')
  parser.add_argument(
      '--dryrun', type=bool, nargs='?', const=True, default=False,
      help='print out goldctl invocations, do not actually run goldctl')
  args = parser.parse_args()

  # Either none or all of these flags should be set.
  required_all_or_none = [args.issue, args.patch_set, args.task_id]
  if not (all(v is None for v in required_all_or_none) or
          all(v is not None for v in required_all_or_none)):
    sys.stderr.write(
        'Please provide either none or all flags --issue, --patch_set and ' +
        '--task_id.\n')
    sys.exit(1)

  # It's a trybot if any of the flags above are set, so we only check one.
  is_trybot = args.issue is not None

  # More validation.
  if args.issue is not None and args.issue < 1:
    sys.stderr.write('Flag --issue must be a positive integer.\n')
    sys.exit(1)
  if args.patch_set is not None and args.patch_set < 1:
    sys.stderr.write('Flag --patch_set must be a positive integer.\n')
    sys.exit(1)

  # Invokes the goldctl binary with the given arguments.
  def goldctl(goldctl_args):
    if args.dryrun:
      print '[DRYRUN] Executing: goldctl %s' % ' '.join(goldctl_args)
    else:
      print 'Executing: goldctl %s' % ' '.join(goldctl_args)
      exit_code = os.system(' '.join([args.path_to_goldctl] + goldctl_args))
      if exit_code != 0:
        sys.exit(exit_code)
    print ''

  # Print out command summary.
  def optional_arg(arg):
    return str(arg) if arg else 'NOT SET'
  print '%s called with the following flags:' % __file__
  print '  --path_to_goldctl: %s' % args.path_to_goldctl
  print '  --revision:        %s' % args.revision
  print '  --issue:           %s' % optional_arg(args.issue)
  print '  --patch_set:       %s' % optional_arg(args.patch_set)
  print '  --task_id:         %s' % optional_arg(args.task_id)
  print ''
  if args.local:
    print 'WARNING: Flag --local passed. Authorizing goldctl with gsutil. ' + \
          'DO NOT USE IN PRODUCTION.'
    print ''

  # Images are assumed to be in the "output" directory alongside this script.
  dirname, _ = os.path.split(os.path.abspath(__file__))
  images_dir = os.path.join(dirname, 'output')

  # Generate keys file.
  _, keys_file = tempfile.mkstemp()
  with open(keys_file, "w") as f:
    f.write('{"source_type": "gold"}\n')  # Corpus.
    f.flush()

  # Authorize goldctl.
  work_dir = tempfile.mkdtemp()
  goldctl(['auth', '--work-dir', work_dir] + ([] if args.local else ['--luci']))

  # Initialize.
  cmd = [
      'imgtest', 'init',
      '--work-dir', work_dir,
      '--instance', 'skia-infra',
      '--commit', args.revision,
      '--keys-file', keys_file
  ]
  if is_trybot:
    cmd += [
        '--crs', 'gerrit',
        '--cis', 'buildbucket',
        '--changelist', str(args.issue),
        '--patchset', str(args.patch_set),
        '--jobid', args.task_id,
    ]
  goldctl(cmd)

  # Add images.
  for filename in os.listdir(images_dir):
    if not filename.lower().endswith('.png'):
      continue
    goldctl([
        'imgtest', 'add',
        '--work-dir', work_dir,
        '--png-file', os.path.join(images_dir, filename),
        '--test-name', filename[:-4], # Remove .png extension.
    ])

  # Finalize and clean up.
  goldctl(['imgtest', 'finalize', '--work-dir', work_dir])
  shutil.rmtree(work_dir)
  os.remove(keys_file)


if __name__ == '__main__':
  main()

#!/usr/bin/env python3

# Ref format:
# refs/intermediates/<tree hash>
# refs/tags/233d08f0fe0116f8b9ff6f8d256f461c15500804+compile

# Setup:
# 

# To download an "isolate":
# $ git fetch origin $REF
# $ git --work-tree=$DEST checkout FETCH_HEAD -- .

# To upload an "isolate" (DEST must be an absolute path):
# NOTE: This assumes that the ref does not already exist in the remote.
# $ git --work-tree=$DEST checkout --orphan $REF
# $ git --work-tree=$DEST add -f <files>
# $ git --work-tree=$DEST commit --no-verify -m "$REF"
# $ TREE=$(git rev-parse $REF^{tree})
# $ git push --set-upstream origin refs/intermediates/$TREE

import ast
import os
import random
import shutil
import subprocess
import sys


def ref(tree_hash):
  return 'refs/intermediates/%s' % tree_hash


def download(git_dir, dest_dir, tree_hash):
  """Download the given tree hash to the given destination."""
  subprocess.check_call(['git', 'fetch', 'origin', ref(tree_hash)], cwd=git_dir)
  subprocess.check_call(['git', '--work-tree=%s' % dest_dir, 'checkout', 'FETCH_HEAD'], cwd=git_dir)


def upload(git_dir, src_dir):
  """Upload the given source dir. Returns the tree hash."""
  wt = '--work-tree=%s' % src_dir
  branch = 'tmp-%d' % random.randint(10000, 99999)
  def git(*args):
    return subprocess.check_output(['git', wt] + args, cwd=git_dir)

  git('checkout', '--orphan', branch)
  git('add', '-f', '.')
  git('commit', '--no-verify', '-m', 'blah blah')
  tree_hash = git('rev-parse', branch+'^{tree}').rstrip()
  # Check for the ref's existence in the remote. If it exists, don't push.
  if tree_hash not in git('ls-remote', 'origin', ref(tree_hash)):
    git('push', 'origin', '%s:%s' % (branch, ref(tree_hash)))
  return tree_hash


def load_isolate(root, isolate):
  base = os.path.dirname(os.path.relpath(isolate, root))
  with open(isolate) as f:
    content = ast.literal_eval(f.read())
  files = set()
  for f in content.get('variables', {}).get('files', []):
    files.add(os.path.normpath(os.path.join(base, f)))
  for inc in content.get('includes', []):
    for f in load_isolate(root, os.path.join(base, inc)):
      files.add(f)
  return sorted(files)


def main():
  if len(sys.argv) < 2:
    print('USAGE: %s cmd ...' % sys.argv[0])
    sys.exit(1)

  cmd = sys.argv[1]
  args = sys.argv[1:]
  if cmd == 'download':
    if len(args) != 3:
      print('USAGE: %s %s <git-dir> <tree-hash> <dest-dir>' % (sys.argv[0], cmd))
    git_dir = args[0]
    tree_hash = args[1]
    dest_dir = args[2]
    download(git_dir, dest_dir, tree_hash)
  else if cmd == 'upload':
    if len(args) != 2:
      print('USAGE: %s %s <git-dir> <src-dir>' % (sys.argv[0], cmd))
    git_dir = args[0]
    src_dir = args[1]
    tree_hash = upload(git_dir, src_dir)
    print(tree_hash)
  else if cmd == 'isolate':
    pass
  root = sys.argv[1]
  isolate = sys.argv[2]

  # TODO(borenet): Don't depend on current repo state.
  head = subprocess.check_output(['git', 'rev-parse', 'HEAD']).rstrip()
  ref = head + '-' + os.path.basename(isolate).split('.')[0]
  subprocess.check_call(['git', 'checkout', '--orphan', ref])
  # We have a bunch of files from upstream staged; unstage them.
  subprocess.check_call(['git', 'reset'])
  files = load_isolate(root, isolate)
  for f in files:
    if not f.startswith('..'):
      subprocess.check_call(['git', 'add', '-f', f])
  subprocess.check_call(['git', 'commit', '--no-verify', '-m', '%s from %s' % (os.path.basename(isolate), head)])
  subprocess.check_call(['git', 'push', '--set-upstream', 'origin', ref])


if __name__ == '__main__':
  main()

#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Verify that bots are using the correct Git credentials."""


import fnmatch
import json
import netrc
import os
import socket
import subprocess
import urllib2


# Collect errors and report at the end.
errors = []


# First, ensure that .gitconfig and .netrc exists, and no .gitcookies exist.
home = os.path.expanduser('~')
expected_locations = [home]
netrc_file = '.netrc'
if os.name == 'nt':
  netrc_file = '_netrc'
expected_files = ['.gitconfig', netrc_file]
unexpected_files = ['.gitcookies']

for loc in expected_locations:
  for f in expected_files:
    path = os.path.join(loc, f)
    if not os.path.isfile(path):
      errors.append('Missing: %s' % path)
  for f in unexpected_files:
    path = os.path.join(loc, f)
    if os.path.isfile(path):
      errors.append('Found unexpected file %s' % path)


# Verify that each checkout uses an authenticated remote URL.
def get_remote_url(git_dir):
  """Return the remote URL for the given git dir. Follow local upstreams."""
  remote = subprocess.check_output([
      'git', '--git-dir', git_dir, 'remote', 'get-url', 'origin']).rstrip()
  if remote.startswith('http'):
    return remote
  return get_remote_url(remote)


def uses_authenticated_endpoint(git_dir):
  """Return true iff the remote URL uses the authenticated endpoint."""
  remote_url = get_remote_url(git_dir)
  # Make an exception for non-googlesource repos.
  # TODO(borenet): These should really be mirrored...
  if 'googlesource' in remote_url:
    return '.com/a/' in remote_url
  return True


workdir = '/b/work'
if os.name == 'nt':
  workdir = 'c:\\b\\work'

git_dirs = []
for r, dirs, files in os.walk(workdir):
  for d in dirs:
    if d == '.git' and '.recipe_deps' not in r:  # Recipe DEPS have no origin?
      git_dirs.append(os.path.join(r, d))

for d in git_dirs:
  if not uses_authenticated_endpoint(d):
    errors.append('%s does not use authenticated endpoint!' % d)


# Now, verify the .netrc. Requires 'skia-review.googlesource.com' in .netrc.
expect_account = 'bots@skia.org'
if '-i-' in socket.gethostname():
  expect_account = 'bots-internal@skia.org'

n = netrc.netrc()
host = 'skia-review.googlesource.com'
auths = n.authenticators(host)
if not auths:
  errors.append('No .netrc entry for %s' % host)
else:
  user, _, password = auths
  mgr = urllib2.HTTPPasswordMgrWithDefaultRealm()
  mgr.add_password(None, host, user, password)
  handler = urllib2.HTTPBasicAuthHandler(mgr)
  opener = urllib2.build_opener(handler)
  resp = opener.open(
      'https://skia-review.googlesource.com/a/accounts/self').read()
  prefix = ')]}\'\n'
  if resp.startswith(prefix):
    resp = resp[len(prefix):]

  j = json.loads(resp)
  account = j.get('email')
  if account != expect_account:
    errors.append('Expected account %s but got %s' % (expect_account, account))


if len(errors) != 0:
  raise Exception('Encountered errors:\n\n%s' % ('\n'.join(errors)))


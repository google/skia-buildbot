# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Runs a series of tests on a file which maps old builder names to new builder
names. """


import json
import os
import sys


EXPECTED_DIR = os.path.join(os.path.dirname(__file__), 'expected')


def VerifyMapping(old_name, new_name, slaves_cfg, mapping):
  """ Run assertions to verify that the mapping from an old builder name to a
  new builder name is valid. """
  try:
    # First, is the old name actually a builder name?
    assert os.path.isfile(os.path.join(EXPECTED_DIR, old_name))

    # Trybots should maintain the trybot suffix.
    if old_name.endswith('Trybot'):
      assert new_name.endswith('Trybot')
    else:
      # If the builder isn't a trybot, it should be explicitly listed in
      # slaves.cfg.
      assert new_name in slaves_cfg

    # Assert that the new builder name isn't duplicated.
    match_count = 0
    for val in mapping.itervalues():
      if val == new_name:
        match_count += 1
    assert match_count == 1

    # ADD YOUR ASSERTIONS HERE

  except Exception:
    print 'Failed assertion for (%s to %s)' % (old_name, new_name)
    raise


def main(argv):
  if len(argv) != 2:
    raise Exception('Invalid arguments given; you must provide a mapping file.')
  with open(argv[1]) as f:
    mapping = json.load(f)

  slaves_cfg_file = os.path.join('master', 'slaves.cfg')
  with open(slaves_cfg_file) as f:
    slaves_cfg = f.read()

  for old_name, new_name in mapping.iteritems():
    VerifyMapping(old_name, new_name, slaves_cfg, mapping)


if '__main__' == __name__:
  sys.exit(main(sys.argv))
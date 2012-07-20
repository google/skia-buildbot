# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Miscellaneous utilities needed by the Skia buildbot master."""

def FileBug(summary, description, owner=None, ccs=[], labels=[]):
  """Files a bug to the Skia issue tracker.

  Args:
    summary: a single-line string to use as the issue summary
    description: a multiline string to use as the issue description
    owner: email address of the issue owner (as a string), or None if unknown
    ccs: email addresses (list of strings) to CC on the bug
    labels: labels (list of strings) to apply to the bug
  """
  # TODO: for now, this is a skeletal implementation to aid discussion of
  # https://code.google.com/p/skia/issues/detail?id=726
  # ('buildbot: automatically file bug report when the build goes red')
  tracker_base_url = 'https://code.google.com/p/skia/issues'
  reporter = 'user@domain.com'  # I guess we'll need to set up an account for this purpose
  credentials = None    # presumably we will need credentials to log in as reporter;
                        # note that the credentials should not be included in the source code!
  # Code goes here...

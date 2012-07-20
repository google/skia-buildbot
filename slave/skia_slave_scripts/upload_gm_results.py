#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload actual GM results to the skia-autogen SVN repository to aid in rebaselining.

TODO: Move per-buildstep scripts like this into their own "steps" directory.

To test:
  cd .../buildbot/slave/skia_slave_scripts
  echo "SvnUsername" >../../site_config/.autogen_svn_username
  echo "SvnPassword" >../../site_config/.autogen_svn_password
  SUBDIR=test
  DIR=../../gm/actual/$SUBDIR
  rm -rf $DIR
  mkdir -p $DIR
  date >$DIR/date.png
  date >$DIR/date.txt
  CR_BUILDBOT_PATH=../../third_party/chromium_buildbot
  PYTHONPATH=$CR_BUILDBOT_PATH/scripts:$CR_BUILDBOT_PATH/site_config \
   python upload_gm_results.py \
   --builder_name=$USER \
   --gm_image_subdir=$SUBDIR \
   --got_revision=$(date +%s)
  # and check
  #  http://code.google.com/p/skia-autogen/source/browse/#svn%2Fgm-actual%2Ftest

"""

import merge_into_svn
import optparse
import os
import skia_slave_utils
import sys

# Class we can set attributes on, to emulate optparse-parsed options.
# TODO: Remove the need for this by passing parameters into MergeIntoSvn() some other way
class Options(object):
  pass

def UploadGMResults(options):
  # TODO these constants should actually be shared by multiple build steps
  gm_actual_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'actual')
  gm_merge_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'merge')
  autogen_svn_baseurl = 'https://skia-autogen.googlecode.com/svn'
  gm_actual_svn_baseurl = '%s/%s' % (autogen_svn_baseurl, 'gm-actual')
  autogen_svn_username_file = '.autogen_svn_username'
  autogen_svn_password_file = '.autogen_svn_password'

  # Call MergeIntoSvn to actually perform the work.
  # TODO: We should do something a bit more sophisticated, to address
  # https://code.google.com/p/skia/issues/detail?id=720 ('UploadGMs step should be skipped when
  # re-running old revisions of the buildbot')
  merge_options = Options()
  merge_options.commit_message = 'UploadGMResults of r%s on %s' % (
      options.got_revision, options.builder_name)
  merge_options.dest_svn_url = '%s/%s' % (gm_actual_svn_baseurl, options.gm_image_subdir)
  merge_options.merge_dir_path = os.path.join(gm_merge_basedir, options.gm_image_subdir)
  merge_options.source_dir_path = os.path.join(gm_actual_basedir, options.gm_image_subdir)
  merge_options.svn_password_file = autogen_svn_password_file
  merge_options.svn_username_file = autogen_svn_username_file
  merge_into_svn.MergeIntoSvn(merge_options)

def main(argv):
  option_parser = optparse.OptionParser()
  option_parser.add_option('--builder_name', help='e.g. Skia_Linux_Float_Debug')
  option_parser.add_option('--gm_image_subdir', help='e.g. base-linux')
  option_parser.add_option('--got_revision', help='e.g. 4321')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  skia_slave_utils.ConfirmOptionsSet({
      '--builder_name': options.builder_name,
      '--gm_image_subdir': options.gm_image_subdir,
      '--got_revision': options.got_revision,
      })
  return UploadGMResults(options)

if '__main__' == __name__:
  sys.exit(main(None))

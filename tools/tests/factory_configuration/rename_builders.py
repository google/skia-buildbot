# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Changes builder names in the expected factory configurations. """


import os
import json
import re
import subprocess
import sys


EXPECTED_DIR = os.path.join(os.path.dirname(__file__), 'expected')
if os.name == 'nt':
  SVN = 'svn.bat'
else:
  SVN = 'svn'


def CheckName(builder_name):
  """ Make sure that the builder name contains no illegal characters. """
  if not re.match('^[\w\-_\.]+$', builder_name):
    raise ValueError('"%s" is not a valid builder name.' % builder_name)


def ExistingBuilders():
  """ List the existing set of builder names. """
  builders = os.listdir(EXPECTED_DIR)
  builders.sort()
  for builder_file in builders:
    if not builder_file.startswith('.'):
      yield builder_file


def CreateMappingTemplate(mapping_file):
  """ Creates a JSON file containing a template for mapping old builder names to
  new builder names. """
  mapping = {}
  for builder_file in ExistingBuilders():
    mapping[builder_file] = ''
  with open(mapping_file, 'w') as f:
    json.dump(mapping, f, indent=4, sort_keys=True)
  print 'Created mapping template file in %s' % mapping_file


def Rename(old_name, new_name):
  """ "Rename" a builder. Runs "svn mv" to create the new expectations file,
  then replaces all instances of the old builder name with the new in that file.
  """
  print 'Changing "%s" to "%s".' % (old_name, new_name)
  old_file_path = os.path.join(EXPECTED_DIR, old_name)
  new_file_path = os.path.join(EXPECTED_DIR, new_name)

  # Verify the new file names.
  CheckName(old_name)
  CheckName(new_name)
  if not os.path.isfile(old_file_path):
    raise Exception('Config file for "%s" does not exist!' % old_file_path)
  if os.path.isfile(new_file_path):
    raise Exception('Config file for "%s" already exists!' % new_file_path)

  # Read the old builder configuration.
  with open(old_file_path) as old_file:
    old_file_contents = old_file.read()

  # Use "svn mv" to create the new file so that the diff only shows the changes.
  subprocess.call([SVN, 'mv', old_file_path, new_file_path])

  # Replace any instances of old_name with new_name in file_contents.
  new_file_contents = old_file_contents.replace(old_name, new_name)

  # Write the new builder configuration.
  with open(new_file_path, 'w') as new_file:
    new_file.write(new_file_contents)


def Usage(err_msg=None):
  if err_msg:
    print 'Error: %s\n' % err_msg
  print """rename_builders.py: Rename builders and update expectations files.

Options:
  -h, --help                     Display this message.
  -t, --create-mapping-template  Create a JSON template file for the mapping.

Example usage:

Create a mapping template file to be filled in manually. File name is optional,
default is "mapping.json".
$ rename_builders.py -t [filename]

Map builders using a mapping file.
$ rename_builders.py <filename>

Interactive. Given each old builder name, prompts for the new name.
$ rename_builders.py

"""
  sys.exit(1)


def main(argv):
  if len(argv) > 1:
    if argv[1] == '--help' or argv[1] == '-h':
      Usage()
    if argv[1] == '--create-mapping-template' or argv[1] == '-t':
      if len(argv) == 3:
        mapping_file = argv[2]
      elif len(argv) == 2:
        mapping_file = 'mapping.json'
      else:
        Usage('Too many arguments provided.')
      CreateMappingTemplate(mapping_file)
      return
    else:
      if len(argv) != 2:
        Usage('Too many arguments provided.')
      if not os.path.isfile(argv[1]):
        Usage('Please provide a mapping file.')
      with open(argv[1]) as f:
        mapping = json.load(f)
      for builder_file in ExistingBuilders():
        if not builder_file in mapping:
          raise Exception('Your mapping file contains no mapping for "%s"' % \
                              builder_file)
  else:
    print ('No mapping file provided. Next time you can provide a '
           'JSON-formatted file which maps old names to new names.')
    mapping = {}
    for builder_file in ExistingBuilders():
      new_name = raw_input('New name for %s: ' % builder_file)
      mapping[builder_file] = new_name
      break

  for old_name, new_name in mapping.iteritems():
    Rename(old_name, new_name)


if '__main__' == __name__:
  sys.exit(main(sys.argv))
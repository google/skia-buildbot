#!/usr/bin/python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


import json
import os
import sys


GLOBAL_VARIABLES_FILE = (os.path.join(os.path.dirname(__file__),
                                      'global_variables.json'))


with open(GLOBAL_VARIABLES_FILE) as f:
  _GLOBAL_VARIABLES = json.load(f)


def GetGlobalVariable(var_name):
  """ Retrieve the requested variable from the JSON file.

  var_name: string; name of the variable to retrieve.
  """
  if var_name not in _GLOBAL_VARIABLES:
    raise Exception('Unknown variable name: %s' % var_name)
  var_type = _GLOBAL_VARIABLES[var_name]['type']
  if var_type == 'integer':
    return int(_GLOBAL_VARIABLES[var_name]['value'])
  elif var_type == 'boolean':
    return bool(_GLOBAL_VARIABLES[var_name]['value'])
  elif var_type == 'string':
    return str(_GLOBAL_VARIABLES[var_name]['value'])
  else:
    raise Exception('Unknown variable type: %s' % var_type)


def main(argv):
  if len(argv) != 1:
    print 'No variable name provided!'
    return 1
  print GetGlobalVariable(argv[0])
  return 0


if __name__ == '__main__':
  sys.exit(main(sys.argv[1:]))

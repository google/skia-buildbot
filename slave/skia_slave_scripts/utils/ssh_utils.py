#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools related to ssh used by the buildbot scripts. """

import shell_utils

def PutSCP(local_path, remote_path, username, host, port, recurse=False):
  """ Send a file to the given host over SCP. Assumes that public key
  authentication is set up between the client and server.

  local_path: path to the file to send on the client
  remote_path: destination path for the file on the server
  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  recurse: boolean indicating whether to transmit everything in a folder
  """
  # TODO: This will hang for a while if the host does not recognize the client
  cmd = ['scp']
  if recurse:
    cmd += ['-r']
  cmd += ['-P', port, local_path, '%s@%s:%s' % (username, host, remote_path)]
  shell_utils.Bash(cmd)

def GetSCP(local_path, remote_path, username, host, port, recurse=False):
  """ Retrieve a file from the given host over SCP. Assumes that public key
  authentication is set up between the client and server.

  local_path: destination path for the file on the client
  remote_path: path to the file to retrieve on the server
  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  recurse: boolean indicating whether to transmit everything in a folder
  """
  # TODO: This will hang for a while if the host does not recognize the client
  cmd = ['scp']
  if recurse:
    cmd += ['-r']
  cmd += ['-P', port, '%s@%s:%s' % (username, host, remote_path), local_path]
  shell_utils.Bash(cmd)

def RunSSH(username, host, port, command, echo=True):
  """ Login to the given host and run the given command.
  
  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  command: command to run on the server in list format
  """
  # TODO: This will hang for a while if the host does not recognize the client
  return shell_utils.Bash(['ssh', '-p', port, '%s@%s' % (username, host),
                           '%s' % (' '.join(command).replace('"', '\"'))],
                          echo=echo)
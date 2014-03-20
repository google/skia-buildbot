#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools related to ssh used by the buildbot scripts. """

import atexit
import os
import re
import shell_utils
import signal

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
  shell_utils.run(cmd)


def MultiPutSCP(local_paths, remote_path, username, host, port):
  """ Send files to the given host over SCP. Assumes that public key
  authentication is set up between the client and server.

  local_paths: list of paths of files and directories to send on the client
  remote_path: destination directory path on the server
  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  """
  # TODO: This will hang for a while if the host does not recognize the client
  target = '%s@%s:%s' % (username, host, remote_path)
  cmd = ['scp', '-r', '-P', port]
  cmd += local_paths
  cmd.append(target)
  shell_utils.run(cmd)


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
  shell_utils.run(cmd)


def RunSSHCmd(username, host, port, command, echo=True):
  """ Login to the given host and run the given command.

  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  command: (string) command to run on the server
  """
  # TODO: This will hang for a while if the host does not recognize the client
  return shell_utils.run(
    ['ssh', '-p', port, '%s@%s' % (username, host), command], echo=echo)


def ShellEscape(arg):
  """ Escape a single argument for passing into a remote shell
  """
  arg = re.sub(r'(["\\])', r'\\\1', arg)
  return '"%s"' % arg if re.search(r'[\' \t\r\n]', arg) else arg


def RunSSH(username, host, port, command, echo=True):
  """ Login to the given host and run the given command.

  username: ssh login name
  host: hostname or ip address of the server
  port: port on the server to use
  command: command to run on the server in list format
  """
  cmd = ' '.join(ShellEscape(arg) for arg in command)
  return RunSSHCmd(username, host, port, cmd, echo=echo)


class SshDestination(object):
  """ Convenience class to remember a host, port, and username.
  Wraps the other functions in this module.
  """
  def __init__(self, host, port, username):
    """
    host - (string) hostname of the target
    port - (string or int) sshd port on the target
    username - (string) remote username
    """
    self.host = host
    self.port = str(port)
    self.user = username

  def Put(self, local_path, remote_path, recurse=False):
    return PutSCP(
      local_path, remote_path, self.user,
      self.host, self.port, recurse=recurse)

  def MultiPut(self, local_paths, remote_path):
    return MultiPutSCP(
      local_paths, remote_path, self.user, self.host, self.port)

  def Get(self, local_path, remote_path, recurse=False):
    return GetSCP(
      local_path, remote_path, self.user,
      self.host, self.port, recurse=recurse)

  def RunCmd(self, command, echo=True):
    return RunSSHCmd(self.user, self.host, self.port, command, echo=echo)

  def Run(self, command, echo=True):
    return RunSSH(self.user, self.host, self.port, command, echo=echo)


def search_within_string(input_string, pattern):
  """Search for regular expression in a string.

  input_string: (string) to be searched
  pattern: (string) to be passed to re.compile, with a symbolic
           group named 'return'.
  default: what to return if no match

  Returns a string or None
  """
  match = re.search(pattern, input_string)
  return match.group('return') if match else None

def SSHAdd(key_file):
  """ Call ssh-add, and call ssh-agent if necessary.
  """
  assert os.path.isfile(key_file)
  try:
    shell_utils.run(['ssh-add', key_file],
                    log_in_real_time=False)
    return
  except shell_utils.CommandFailedException:
    ssh_agent_output = shell_utils.run(['ssh-agent', '-s'],
                                       log_in_real_time=False)
    if not ssh_agent_output:
      raise Exception('ssh-agent did not print anything')
    ssh_auth_sock = search_within_string(
      ssh_agent_output, r'SSH_AUTH_SOCK=(?P<return>[^;]*);')
    ssh_agent_pid = search_within_string(
      ssh_agent_output, r'SSH_AGENT_PID=(?P<return>[^;]*);')
    if not (ssh_auth_sock and ssh_agent_pid):
      raise Exception('ssh-agent did not print meaningful data')
    os.environ['SSH_AUTH_SOCK'] = ssh_auth_sock
    os.environ['SSH_AGENT_PID'] = ssh_agent_pid
    atexit.register(os.kill, int(ssh_agent_pid), signal.SIGTERM)
    shell_utils.run(['ssh-add', key_file])

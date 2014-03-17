#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run a command and report its results in machine-readable format."""


import collections
import optparse
import os
import pickle
import pprint
import socket
import subprocess
import sys
import traceback

buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path))

from site_config import slave_hosts_cfg


class BaseCommandResults(object):
  """Base class for CommandResults classes."""

  # We print this string before and after the important output from the command.
  # This makes it easy to ignore output from SSH, shells, etc.
  BOOKEND_STR = '@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@'

  def encode(self):
    """Convert the results into a machine-readable string.

    Returns:
        A hex-encoded string, bookended by BOOKEND_STR for easy parsing.
    """
    raise NotImplementedError()

  @staticmethod
  def decode(results_str):
    """Convert a machine-readable string into a CommandResults instance.

    Args:
        results_str: string; output from "run" or one of its siblings.
    Returns:
        A dictionary of results.
    """
    decoded_dict = pickle.loads(
        results_str.split(BaseCommandResults.BOOKEND_STR)[1].decode('hex'))
    errors = []
    # First, try to interpret the dict as SingleCommandResults.
    try:
      # This will fail unless decoded_dict has the following set of keys:
      # ('returncode', 'stdout', 'stderr')
      return SingleCommandResults(**decoded_dict)
    except TypeError:
      errors.append(traceback.format_exc())
    # Next, try to interpret the dict as MultiCommandResults.
    try:
      results_dict = {}
      for (slavename, results) in decoded_dict.iteritems():
        results_dict[slavename] = BaseCommandResults.decode(results)
      return MultiCommandResults(results_dict)
    except Exception:
      errors.append(traceback.format_exc())
    raise Exception('Unable to decode CommandResults from dict:\n\n%s\n%s'
                    % ('\n'.join(errors), decoded_dict))

  def print_results(self, pretty=False):
    """Print the results of a command.

    Args:
        pretty: bool; whether or not to print in human-readable format.
    """
    if pretty:
      print pprint.pformat(self.__dict__)
    else:
      print self.encode()


class SingleCommandResults(collections.namedtuple('CommandResults_tuple',
                                                  'stdout, stderr, returncode'),
                           BaseCommandResults):
  """Results for a single command. Properties: stdout, stderr, and returncode"""

  def encode(self):
    """Convert the results into a machine-readable string.

    Returns:
        A hex-encoded string, bookended by BOOKEND_STR for easy parsing.
    """
    return (BaseCommandResults.BOOKEND_STR +
            pickle.dumps(self.__dict__).encode('hex') +
            BaseCommandResults.BOOKEND_STR)

  @staticmethod
  def make(stdout='', stderr='', returncode=1):
    """Create CommandResults for a command.

    Args:
        stdout: string; stdout from a command.
        stderr: string; stderr from a command.
        returncode: string; return code of a command.
    """
    return SingleCommandResults(stdout=stdout,
                                stderr=stderr,
                                returncode=returncode)

  @property
  def __dict__(self):
    """Return a dictionary representation of this CommandResults instance.

    Since collections.NamedTuple.__dict__ returns an OrderedDict, we have to
    create this wrapper to get a normal dict.
    """
    return dict(self._asdict())


class MultiCommandResults(BaseCommandResults):
  """Encapsulates CommandResults for multiple buildslaves or hosts.

  MultiCommandResults can form tree structures whose leaves are instances of
  SingleComamandResults and interior nodes are instances of MultiCommandResults:

  MultiCommandResults({
      'remote_slave_host_name': MultiCommandResults({
              'slave_name': SingleCommandResults,
              'slave_name2': SingleCommandResults,
          }),
      'local_slave_name': SingleCommandResults,
  })
  """

  def __init__(self, results):
    """Instantiate the MultiCommandResults.

    Args:
        results: dict whose keys are slavenames or slave host names and values
            are instances of a BaseCommandResults subclass.
    """
    super(MultiCommandResults, self).__init__()
    self._dict = {}
    for (slavename, result) in results.iteritems():
      if not issubclass(result.__class__, BaseCommandResults):
        raise ValueError('%s is not a subclass of BaseCommandResults.'
                         % result.__class__)
      self._dict[slavename] = result

  def __getitem__(self, key):
    return self._dict[key]

  def encode(self):
    """Convert the results into a machine-readable string.

    Returns:
        A hex-encoded string, bookended by BOOKEND_STR for easy parsing.
    """
    encoded_dict = dict([(key, value.encode())
                         for (key, value) in self._dict.iteritems()])
    return (BaseCommandResults.BOOKEND_STR +
            pickle.dumps(encoded_dict).encode('hex') +
            BaseCommandResults.BOOKEND_STR)

  @property
  def __dict__(self):
    return dict([(key, value.__dict__)
                 for (key, value) in self._dict.iteritems()])


class ResolvableCommandElement(object):
  """Base class for elements of commands which have different string values
  depending on the properties of the host."""

  def resolve(self, slave_host_name):
    """Resolve this ResolvableCommandElement as appropriate.

    Args:
        slave_host_name: string; name of the slave host.
    Returns:
        string whose value depends on the given slave_host_name in some way.
    """
    raise NotImplementedError


class BuildbotPath(ResolvableCommandElement):
  """Path to the buildbot scripts checkout on a slave host machine."""

  def resolve(self, slave_host_name):
    """Return the resolved path to the buildbot checkout on a slave_host.

    Args:
        slave_host_name: string; name of the slave host.
    Returns:
        string; the path to the buildbot checkout.
    """
    host_data = slave_hosts_cfg.get_slave_host_config(slave_host_name)
    return host_data.path_module.join(*host_data.path_to_buildbot)


class ResolvablePath(ResolvableCommandElement):
  """Represents a path."""

  def __init__(self, *path_elems):
    """Instantiate this ResolvablePath.

    Args:
        path_elems: strings or ResolvableCommandElements which will be joined to
            form a path.
    """
    super(ResolvablePath, self).__init__()
    self._path_elems = list(*path_elems)

  def resolve(self, slave_host_name):
    """Resolve this ResolvablePath as appropriate.

    Args:
        slave_host_name: string; name of the slave host.
    Returns:
        string whose value depends on the given slave_host_name in some way.
    """
    host_data = slave_hosts_cfg.get_slave_host_config(slave_host_name)
    fixed_path_elems = _fixup_cmd(self._path_elems, slave_host_name)
    return host_data.path_module.join(*fixed_path_elems)

  @staticmethod
  def buildbot_path(*path_elems):
    """Convenience method; returns a path relative to the buildbot checkout."""
    return ResolvablePath([BuildbotPath()] + list(path_elems))


def _fixup_cmd(cmd, slave_host_name):
  """Resolve the command into a list of strings.

  Args:
      cmd: list containing strings or ResolvableCommandElements.
      slave_host_name: string; the name of the relevant slave host machine.
  """
  new_cmd = []
  for elem in cmd:
    if isinstance(elem, ResolvableCommandElement):
      resolved_elem = elem.resolve(slave_host_name)
      new_cmd.append(resolved_elem)
    else:
      new_cmd.append(elem)
  return new_cmd


def _launch_cmd(cmd, cwd=None):
  """Launch the given command. Non-blocking.

  Args:
      cmd: list of strings; command to run.
      cwd: working directory in which to run the process. Defaults to the root
          of the buildbot checkout containing this file.
  Returns:
      subprocess.Popen instance.
  """
  if not cwd:
    cwd = buildbot_path
  return subprocess.Popen(cmd, shell=False, cwd=cwd, stderr=subprocess.PIPE,
                          stdout=subprocess.PIPE)


def _get_result(popen):
  """Get the results from a running process. Blocks until the process completes.

  Args:
      popen: subprocess.Popen instance.
  Returns:
      CommandResults instance, decoded from the results of the process.
  """
  stdout, stderr = popen.communicate()
  try:
    return BaseCommandResults.decode(stdout)
  except Exception:
    pass
  return SingleCommandResults.make(stdout=stdout,
                                   stderr=stderr,
                                   returncode=popen.returncode)


def run(cmd):
  """Run the command, block until it completes, and return a results dictionary.

  Args:
      cmd: string or list of strings; the command to run.
  Returns:
      CommandResults instance, decoded from the results of the command.
  """
  try:
    proc = _launch_cmd(cmd)
  except OSError as e:
    return SingleCommandResults.make(stderr=str(e))
  return _get_result(proc)


def run_on_local_slaves(cmd):
  """Run the command on each local buildslave, blocking until completion.

  Args:
      cmd: list of strings; the command to run.
  Returns:
      MultiCommandResults instance containing the results of the command on each
      of the local slaves.
  """
  slave_host = slave_hosts_cfg.get_slave_host_config(socket.gethostname())
  slaves = slave_host.slaves
  results = {}
  procs = []
  for (slave, _) in slaves:
    try:
      proc = _launch_cmd(cmd, cwd=os.path.join(buildbot_path, slave,
                                               'buildbot'))
      procs.append((slave, proc))
    except OSError as e:
      results[slave] = SingleCommandResults.make(stderr=str(e))

  for slavename, proc in procs:
    results[slavename] = _get_result(proc)

  return MultiCommandResults(results)


def _launch_on_remote_host(slave_host_name, cmd):
  """Launch the command on a remote slave host machine. Non-blocking.

  Args:
      slave_host_name: string; name of the slave host machine.
      cmd: list of strings; command to run.
  Returns:
      subprocess.Popen instance.
  """
  host = slave_hosts_cfg.SLAVE_HOSTS[slave_host_name]
  login_cmd = host.login_cmd
  if not login_cmd:
    raise ValueError('%s does not have a remote login procedure defined in '
                     'slave_hosts_cfg.py.' % slave_host_name)
  path_to_buildbot = host.path_module.join(*host.path_to_buildbot)
  path_to_run_cmd = host.path_module.join(path_to_buildbot, 'scripts',
                                          'run_cmd.py')
  return _launch_cmd(login_cmd + ['python', path_to_run_cmd] +
                     _fixup_cmd(cmd, slave_host_name))


def run_on_remote_host(slave_host_name, cmd):
  """Run a command on a remote slave host machine, blocking until completion.

  Args:
      slave_host_name: string; name of the slave host machine.
      cmd: list of strings or ResolvableCommandElements; the command to run.
  Returns:
      CommandResults instance containing the results of the command.
  """
  proc = _launch_on_remote_host(slave_host_name, cmd)
  return _get_result(proc)


def _get_remote_slaves_cmd(cmd):
  """Build a command which runs the command on all slaves on a remote host.

  Args:
      cmd: list of strings or ResolvableCommandElements; the command to run.
  Returns:
      list of strings or ResolvableCommandElements; a command which results in
      the given command being run on all of the slaves on the remote host.
  """
  return ['python',
          ResolvablePath.buildbot_path('scripts',
                                       'run_on_local_slaves.py')] + cmd


def run_on_remote_slaves(slave_host_name, cmd):
  """Run a command on each buildslave on a remote slave host machine, blocking
  until completion.

  Args:
      slave_host_name: string; name of the slave host machine.
      cmd: list of strings or ResolvableCommandElements; the command to run.
  Returns:
      MultiCommandResults instance with results from each slave on the remote
      host.
  """
  proc = _launch_on_remote_host(slave_host_name, _get_remote_slaves_cmd(cmd))
  return _get_result(proc)


def run_on_all_slave_hosts(cmd):
  """Run the given command on all slave hosts, blocking until all complete.

  Args:
      cmd: list of strings or ResolvableCommandElements; the command to run.
  Returns:
      MultiCommandResults instance with results from each remote slave host.
  """
  results = {}
  procs = []

  for hostname in slave_hosts_cfg.SLAVE_HOSTS.iterkeys():
    if not slave_hosts_cfg.SLAVE_HOSTS[hostname].remote_access:
      continue
    if not slave_hosts_cfg.SLAVE_HOSTS[hostname].login_cmd:
      results.update({
          hostname: SingleCommandResults.make(stderr='No procedure for login.'),
      })
    else:
      procs.append((hostname, _launch_on_remote_host(hostname, cmd)))

  for slavename, proc in procs:
    results[slavename] = _get_result(proc)

  return MultiCommandResults(results)


def run_on_all_slaves_on_all_hosts(cmd):
  """Run the given command on all slaves on all hosts. Blocks until completion.

  Args:
      cmd: list of strings or ResolvableCommandElements; the command to run.
  Returns:
      MultiCommandResults instance with results from each slave on each remote
      slave host.
  """
  return run_on_all_slave_hosts(_get_remote_slaves_cmd(cmd))


def parse_args(positional_args=None):
  """Common argument parser for scripts using this module.

  Args:
      positional_args: optional list of strings; extra positional arguments to
          the script.
  """
  parser = optparse.OptionParser()
  parser.disable_interspersed_args()
  parser.add_option('-p', '--pretty', action='store_true', dest='pretty',
                    help='Print output in a human-readable form.')

  # Fixup the usage message to include the positional args.
  cmd = 'cmd'
  all_positional_args = (positional_args or []) + [cmd]
  usage = parser.get_usage().rstrip()
  for arg in all_positional_args:
    usage += ' ' + arg
  parser.set_usage(usage)

  options, args = parser.parse_args()

  # Set positional arguments.
  for positional_arg in positional_args or []:
    try:
      setattr(options, positional_arg, args[0])
    except IndexError:
      parser.print_usage()
      sys.exit(1)
    args = args[1:]

  # Everything else is part of the command to run.
  try:
    setattr(options, cmd, args)
  except IndexError:
    parser.print_usage()
    sys.exit(1)
  return options


if '__main__' == __name__:
  parsed_args = parse_args()
  run(parsed_args.cmd).print_results(pretty=parsed_args.pretty)

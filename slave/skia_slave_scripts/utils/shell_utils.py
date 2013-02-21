#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools for running commands in a shell. """

import os
import Queue
import select
import subprocess
import sys
import threading
import time

if 'nt' in os.name:
  import ctypes


DEFAULT_SECS_BETWEEN_ATTEMPTS = 10
POLL_MILLIS = 250


_all_subprocesses = []
def ListSubprocesses():
  for sub in _all_subprocesses:
    yield sub


def BashAsync(cmd, echo=True, shell=False):
  """ Run 'cmd' in a subprocess, returning a Popen class instance referring to
  that process.  (Non-blocking) """
  if echo:
    print cmd
  if 'nt' in os.name:
    # Windows has a bad habit of opening a dialog when a console program
    # crashes, rather than just letting it crash.  Therefore, when a program
    # crashes on Windows, we don't find out until the build step times out.
    # This code prevents the dialog from appearing, so that we find out
    # immediately and don't waste time waiting around.
    SEM_NOGPFAULTERRORBOX = 0x0002
    ctypes.windll.kernel32.SetErrorMode(SEM_NOGPFAULTERRORBOX)
    flags = 0x8000000 # CREATE_NO_WINDOW
  else:
    flags = 0
  proc = subprocess.Popen(cmd, shell=shell, stderr=subprocess.STDOUT,
                          stdout=subprocess.PIPE, creationflags=flags,
                          bufsize=1)
  _all_subprocesses.append((proc, cmd))
  return proc


class EnqueueThread(threading.Thread):
  """ Reads and enqueues lines from a file. """
  def __init__(self, file_obj, queue):
    threading.Thread.__init__(self)
    self._file = file_obj
    self._queue = queue
    self._stopped = False

  def run(self):
    if sys.platform.startswith('linux'):
      # Use a polling object to avoid the blocking call to readline().
      poll = select.poll()
      poll.register(self._file, select.POLLIN)
      while not self._stopped:
        has_output = poll.poll(POLL_MILLIS)
        if has_output:
          line = self._file.readline()
          if line == '':
            self._stopped = True
          self._queue.put(line)
    else:
      # Only Unix supports polling objects, so just read from the file,
      # Python-style.
      for line in iter(self._file.readline, ''):
        self._queue.put(line)
        if self._stopped:
          break

  def stop(self):
    self._stopped = True


def LogProcessToCompletion(proc, echo=True, timeout=None, log_file=None,
                           halt_on_output=None):
  """ Log the output of proc until it completes. Return a tuple containing the
  exit code of proc and the contents of stdout.

  proc: an instance of Popen referring to a running subprocess.
  echo: boolean indicating whether to print the output received from proc.stdout
  timeout: number of seconds allotted for the process to run
  log_file: an open file for writing output
  halt_on_output: string; kill the process and return if this string is found
      in the output stream from the process.
  """
  stdout_queue = Queue.Queue()
  log_thread = EnqueueThread(proc.stdout, stdout_queue)
  log_thread.start()
  try:
    all_output = []
    t_0 = time.time()
    while True:
      code = proc.poll()
      try:
        output = stdout_queue.get_nowait()
        if echo:
          sys.stdout.write(output)
          sys.stdout.flush()
        if log_file:
          log_file.write(output)
          log_file.flush()
        all_output.append(output)
        if halt_on_output and halt_on_output in output:
          proc.terminate()
          break
      except Queue.Empty:
        if code != None: # proc has finished running
          break
        time.sleep(0.5)
      if timeout and time.time() - t_0 > timeout:
        proc.terminate()
        break
  finally:
    log_thread.stop()
    log_thread.join()
  return (code, ''.join(all_output))


def Bash(cmd, echo=True, shell=False, timeout=None):
  """ Run 'cmd' in a shell and return the combined contents of stdout and
  stderr (Blocking).  Throws an exception if the command exits non-zero.
  
  cmd: list of strings (or single string, iff shell==True) indicating the
      command to run
  echo: boolean indicating whether we should print the command and log output
  shell: boolean indicating whether we are using advanced shell features. Use
      only when absolutely necessary, since this allows a lot more freedom which
      could be exploited by malicious code. See the warning here:
      http://docs.python.org/library/subprocess.html#popen-constructor
  timeout: optional, integer indicating the maximum elapsed time in seconds
  """
  proc = BashAsync(cmd, echo=echo, shell=shell)
  (returncode, output) = LogProcessToCompletion(proc, echo=echo,
                                                timeout=timeout)
  if returncode != 0:
    raise Exception('Command failed with code %d: %s' % (returncode, cmd))
  return output


def BashRetry(cmd, echo=True, shell=False, attempts=1,
              secs_between_attempts=DEFAULT_SECS_BETWEEN_ATTEMPTS,
              timeout=None):
  """ Wrapper for Bash() which makes multiple attempts until either the command
  succeeds or the maximum number of attempts is reached. """
  attempt = 1
  while True:
    try:
      return Bash(cmd, echo=echo, shell=shell, timeout=timeout)
    except Exception:
      if attempt >= attempts:
        raise
    print 'Command failed. Retrying in %d seconds...' % secs_between_attempts
    time.sleep(secs_between_attempts)
    attempt += 1

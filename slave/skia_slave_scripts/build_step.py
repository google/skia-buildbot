# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Base class for all slave-side build steps. """

import config
import errno
import multiprocessing
import os
import shlex
import shutil
import signal
import subprocess
import sys
import time
import traceback

from playback_dirs import LocalSkpPlaybackDirs
from playback_dirs import StorageSkpPlaybackDirs
from utils import file_utils
from utils import misc
from utils import shell_utils


# Add important directories to the PYTHONPATH
buildbot_root = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                             os.pardir, os.pardir)
sys.path.append(os.path.join(buildbot_root, 'site_config'))


DEFAULT_TIMEOUT = 2400
DEFAULT_NO_OUTPUT_TIMEOUT = 3600
DEFAULT_NUM_CORES = 2


GM_EXPECTATIONS_FILENAME = 'expected-results.json'


# multiprocessing.Value doesn't accept boolean types, so we have to use an int.
INT_TRUE = 1
INT_FALSE = 0
build_step_stdout_has_written = multiprocessing.Value('i', INT_FALSE)


# The canned acl to use while copying playback files to Google Storage.
PLAYBACK_CANNED_ACL = 'private'


class BuildStepWarning(Exception):
  pass


class BuildStepFailure(Exception):
  pass


class BuildStepTimeout(Exception):
  pass


class BuildStepLogger(object):
  """ Override stdout so that we can keep track of when anything has been
  logged.  This enables timeouts based on how long the process has gone without
  writing output.
  """
  def __init__(self):
    self.stdout = sys.stdout
    sys.stdout = self
    build_step_stdout_has_written.value = INT_FALSE

  def __del__(self):
    sys.stdout = self.stdout

  def fileno(self):
    return self.stdout.fileno()

  def write(self, data):
    build_step_stdout_has_written.value = INT_TRUE
    self.stdout.write(data)

  def flush(self):
    self.stdout.flush()


class DeviceDirs(object):
  def __init__(self, perf_data_dir, gm_actual_dir, gm_expected_dir,
               resource_dir, skimage_in_dir, skimage_expected_dir,
               skimage_out_dir, skp_dir, skp_perf_dir, skp_out_dir, tmp_dir):
    self._perf_data_dir = perf_data_dir
    self._gm_actual_dir = gm_actual_dir
    self._gm_expected_dir = gm_expected_dir
    self._resource_dir = resource_dir
    self._skimage_in_dir = skimage_in_dir
    self._skimage_expected_dir = skimage_expected_dir
    self._skimage_out_dir = skimage_out_dir
    self._skp_dir = skp_dir
    self._skp_perf_dir = skp_perf_dir
    self._skp_out_dir = skp_out_dir
    self._tmp_dir = tmp_dir

  def GMActualDir(self):
    return  self._gm_actual_dir

  def GMExpectedDir(self):
    return self._gm_expected_dir

  def PerfDir(self):
    return self._perf_data_dir

  def ResourceDir(self):
    return self._resource_dir

  def SKImageInDir(self):
    return self._skimage_in_dir

  def SKImageExpectedDir(self):
    return self._skimage_expected_dir

  def SKImageOutDir(self):
    return self._skimage_out_dir

  def SKPDir(self):
    return self._skp_dir

  def SKPPerfDir(self):
    return self._skp_perf_dir

  def SKPOutDir(self):
    return self._skp_out_dir

  def TmpDir(self):
    return self._tmp_dir


class BuildStep(multiprocessing.Process):

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the associated device. Subclasses should
    override this method with one appropriate for reading the contents of a file
    on the device side. """
    with open(filepath) as f:
      return f.read()

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side. Subclasses should override this method with one appropriate for
    copying the contents of a host-side directory to a clean device-side
    directory."""
    # For "normal" builders who don't have an attached device, we expect
    # host_dir and device_dir to be the same.
    if host_dir != device_dir:
      raise ValueError('For builders who do not have attached devices, copying '
                       'from host to device is undefined and only allowed if '
                       'host_dir and device_dir are the same.')

  def CopyDirectoryContentsToHost(self, device_dir, host_dir):
    """ Copy the contents of a device-side directory to a clean directory on the
    host side. Subclasses should override this method with one appropriate for
    copying the contents of a device-side directory to a clean host-side
    directory."""
    # For "normal" builders who don't have an attached device, we expect
    # host_dir and device_dir to be the same.
    if host_dir != device_dir:
      raise ValueError('For builders who do not have attached devices, copying '
                       'from host to device is undefined and only allowed if '
                       'host_dir and device_dir are the same.')

  def PushFileToDevice(self, src, dst):
    """ Copy the a single file from path "src" on the host to path "dst" on
    the device.  If the host IS the device we are testing, it's just a filecopy.
    Subclasses should override this method with one appropriate for
    pushing the file to the device. """
    shutil.copy(src, dst)

  def DeviceListDir(self, directory):
    """ List the contents of a directory on the connected device. """
    return os.listdir(directory)

  def DevicePathExists(self, path):
    """ Like os.path.exists(), but for a path on the connected device. """
    return os.path.exists(path)

  def DevicePathJoin(self, *args):
    """ Like os.path.join(), but for paths that will target the connected
    device. """
    return os.sep.join(args)

  def CreateCleanDeviceDirectory(self, directory):
    """ Creates an empty directory on an attached device. Subclasses with
    attached devices should override. For builders with no attached device, just
    make sure that the directory exists, since we may want to keep data. """
    # TODO(borenet): This should actually clean the directory, but we don't
    # because we want to avoid deleting historical bench data which we might
    # need.
    try:
      os.makedirs(directory)
    except OSError as e:
      if e.errno != errno.EEXIST:
        raise

  def CreateCleanHostDirectory(self, directory):
    """ Creates an empty directory on the host. Can be overridden by subclasses,
    but that should not be necessary. """
    file_utils.CreateCleanLocalDir(directory)

  def Install(self):
    """ Install the Skia executables. """
    pass

  def Compile(self, target):
    """ Compile the Skia executables. """
    # TODO(borenet): It would be nice to increase code sharing here.
    if 'VS2012' in self._builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
    os.environ['GYP_DEFINES'] = self._args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    cmd = [make_cmd,
           target,
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd.extend(self._default_make_flags)
    cmd.extend(self._make_flags)
    shell_utils.Bash(cmd)

  def __init__(self, args, attempts=1, timeout=DEFAULT_TIMEOUT,
               no_output_timeout=DEFAULT_NO_OUTPUT_TIMEOUT):
    """ Constructs a BuildStep instance.
    
    args: dictionary containing arguments to this BuildStep.
    attempts: how many times to try this BuildStep before giving up.
    timeout: maximum time allowed for this BuildStep.
    no_output_timeout: maximum time allowed for this BuildStep to run without
        any output.
    """
    multiprocessing.Process.__init__(self)
    self._args = args
    self.timeout = timeout
    self.no_output_timeout = no_output_timeout

    self._configuration = args['configuration']
    self._gm_image_subdir = args['gm_image_subdir']
    self._builder_name = args['builder_name']
    self._target_platform = args['target_platform']
    self._deps_target_os = \
        None if args['deps_target_os'] == 'None' else args['deps_target_os']
    self._revision = \
        None if args['revision'] == 'None' or args['revision'] == 'HEAD' \
        else int(args['revision'])
    self._got_revision = \
        None if args['got_revision'] == 'None' else int(args['got_revision'])
    self.attempts = attempts
    self._do_upload_results = (False if args['do_upload_results'] == 'None'
                               else args['do_upload_results'] == 'True')
    # Figure out where we are going to store images generated by GM.
    self._gm_actual_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'actual')
    self._gm_merge_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'merge')
    self._gm_expected_dir = os.path.join(os.pardir, 'gm-expected',
                                         self._gm_image_subdir)
    self._gm_actual_dir = os.path.join(self._gm_actual_basedir,
                                       self._gm_image_subdir)
    self._gm_actual_svn_baseurl = '%s/%s' % (args['autogen_svn_baseurl'],
                                             'gm-actual')
    self._resource_dir = 'resources'
    self._autogen_svn_username_file = '.autogen_svn_username'
    self._autogen_svn_password_file = '.autogen_svn_password'
    self._make_flags = shlex.split(args['make_flags'].replace('"', ''))
    self._test_args = shlex.split(args['test_args'].replace('"', ''))
    self._gm_args = shlex.split(args['gm_args'].replace('"', ''))
    self._gm_args.append('--serialize')
    self._bench_args = shlex.split(args['bench_args'].replace('"', ''))
    self._is_try = args['is_try'] == 'True'

    if os.name == 'nt':
      self._default_make_flags = []
    else:
      # Set the jobs limit to 4, since we have multiple slaves running on each
      # machine.
      self._default_make_flags = ['--jobs', '4', '--max-load=4.0']

    # Adding the playback directory transfer objects.
    self._local_playback_dirs = LocalSkpPlaybackDirs(
        self._builder_name, self._gm_image_subdir,
        None if args['perf_output_basedir'] == 'None'
            else args['perf_output_basedir'])
    self._storage_playback_dirs = StorageSkpPlaybackDirs(
        self._builder_name, self._gm_image_subdir,
        None if args['perf_output_basedir'] == 'None'
            else args['perf_output_basedir'])

    self._skp_dir = self._local_playback_dirs.PlaybackSkpDir()

    # Figure out where we are going to store performance output.
    if args['perf_output_basedir'] != 'None':
      self._perf_data_dir = os.path.join(args['perf_output_basedir'],
                                         self._builder_name, 'data')
      self._perf_graphs_dir = os.path.join(args['perf_output_basedir'],
                                           self._builder_name, 'graphs')
    else:
      self._perf_data_dir = None
      self._perf_graphs_dir = None

    self._skimage_in_dir = os.path.join(os.pardir, 'skimage_in')

    self._skimage_expected_dir = os.path.join('expectations', 'skimage')

    self._skimage_out_dir = os.path.join('out', self._configuration,
                                         'skimage_out')

    # Note that DeviceDirs.GMExpectedDir() is being set up to point at a
    # DIFFERENT directory than self._gm_expected.
    # self._gm_expected : The SVN-managed directory on the buildbot host
    #                     where canonical expectations are stored.
    #                     Currently, they are stored there as
    #                     individual image files.
    # DeviceDirs.GMExpectedDir(): A temporary directory on the device we are
    #                             testing, where the PreRender step will put
    #                             an expected-results.json file that describes
    #                             all GM results expectations.
    # TODO(epoger): Update the above description as we move through the steps in
    # https://goto.google.com/ChecksumTransitionDetail
    self._device_dirs = DeviceDirs(
        perf_data_dir=self._perf_data_dir,
        gm_actual_dir=os.path.join(os.pardir, os.pardir, 'gm', 'actual'),
        gm_expected_dir=os.path.join(os.pardir, os.pardir, 'gm', 'expected'),
        resource_dir=self._resource_dir,
        skimage_in_dir=self._skimage_in_dir,
        skimage_expected_dir=self._skimage_expected_dir,
        skimage_out_dir=self._skimage_out_dir,
        skp_dir=self._local_playback_dirs.PlaybackSkpDir(),
        skp_perf_dir=self._perf_data_dir,
        skp_out_dir=self._local_playback_dirs.PlaybackGmActualDir(),
        tmp_dir=os.path.join(os.pardir, 'tmp'))

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    shell_utils.Bash([self._PathToBinary(app)] + args)

  def _PreRun(self):
    """ Optional preprocessing step for BuildSteps to override. """
    pass

  def _PathToBinary(self, binary):
    """ Returns the path to the given built executable. """
    return os.path.join('out', self._configuration, binary)

  def _Run(self):
    """ Code to be run in a given BuildStep.  No return value; throws exception
    on failure.  Override this method in subclasses.
    """
    raise Exception('Cannot instantiate abstract BuildStep')

  def run(self):
    """ Internal method used by multiprocess.Process. _Run is provided to be
    overridden instead of this method to ensure that this implementation always
    runs.
    """
    # If a BuildStep has exceeded its allotted time, the parent process needs to
    # be able to kill the BuildStep process AND any which it has spawned,
    # without harming itself. On posix platforms, the terminate() method is
    # insufficient; it fails to kill the subprocesses launched by this process.
    # So, we use use the setpgrp() function to set a new process group for the
    # BuildStep process and its children and call os.killpg() to kill the group.
    if os.name == 'posix':
      os.setpgrp()
    try:
      self._Run()
    except BuildStepWarning as e:
      print e
      sys.exit(config.Master.retcode_warnings)

  def _WaitFunc(self, attempt):
    """ Waits a number of seconds depending upon the attempt number of a
    retry-able BuildStep before making the next attempt.  This can be overridden
    by subclasses and should be defined for attempt in [0, self.attempts - 1]

    This default implementation is exponential; we double the wait time with
    each attempt, starting with a 15-second pause between the first and second
    attempts.
    """
    base_secs = 15
    wait = base_secs * (2 ** attempt)
    print 'Retrying in %d seconds...' % wait
    time.sleep(wait)

  @staticmethod
  def KillBuildStep(step):
    """ Kills a running BuildStep.

    step: the running BuildStep instance to kill.
    """
    # On posix platforms, the terminate() method is insufficient; it fails to
    # kill the subprocesses launched by this process. So, we use use the
    # setpgrp() function to set a new process group for the BuildStep process
    # and its children and call os.killpg() to kill the group.
    if os.name == 'posix':
      os.killpg(os.getpgid(step.pid), signal.SIGTERM)
    elif os.name == 'nt':
      subprocess.call(['taskkill', '/F', '/T', '/PID', str(step.pid)])
    else:
      step.terminate()

  @staticmethod
  def RunBuildStep(StepType):
    """ Run a BuildStep, possibly making multiple attempts and handling
    timeouts.
    
    StepType: class type which subclasses BuildStep, indicating what step should
        be run. StepType should override _Run().
    """
    # pylint: disable=W0612
    logger = BuildStepLogger()
    args = misc.ArgsToDict(sys.argv)
    attempt = 0
    while True:
      step = StepType(args=args)
      try:
        start_time = time.time()
        last_written_time = start_time
        # pylint: disable=W0212
        step._PreRun()
        step.start()
        while step.is_alive():
          current_time = time.time()
          if current_time - start_time > step.timeout:
            BuildStep.KillBuildStep(step)
            raise BuildStepTimeout('Build step exceeded timeout of %d seconds' %
                                   step.timeout)
          elif current_time - last_written_time > step.no_output_timeout:
            BuildStep.KillBuildStep(step)
            raise BuildStepTimeout(
                'Build step exceeded %d seconds with no output' %
                step.no_output_timeout)
          time.sleep(1)
          if build_step_stdout_has_written.value == INT_TRUE:
            last_written_time = time.time()
        print 'Build Step Finished.'
        if step.exitcode == 0:
          return 0
        elif step.exitcode == config.Master.retcode_warnings:
          # A warning is considered to be an acceptable finishing state.
          return config.Master.retcode_warnings
        else:
          raise BuildStepFailure('Build step failed.')
      except Exception:
        print traceback.format_exc()
        if attempt + 1 >= step.attempts:
          raise
      # pylint: disable=W0212
      step._WaitFunc(attempt)
      attempt += 1
      print '**** %s, attempt %d ****' % (StepType.__name__, attempt + 1)

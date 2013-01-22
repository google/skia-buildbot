# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side ChromeOS build steps. """


from build_step import BuildStep, DEFAULT_TIMEOUT


class ChromeOSDirs(object):
  def __init__(self, path_prefix):
    self._path_prefix = path_prefix + '/skiabot/skia_'

  def GMDir(self):
    return self._path_prefix + 'gm'

  def PerfDir(self):
    return self._path_prefix + 'perf'

  def SKPDir(self):
    return self._path_prefix + 'skp'

  def SKPPerfDir(self):
    return self._path_prefix + 'skp_perf'

  def SKPOutDir(self):
    return self._path_prefix + 'skp_out'


class ChromeOSBuildStep(BuildStep):
  def __init__(self, args, **kwargs):
    self._ssh_host = args['ssh_host']
    self._ssh_port = args['ssh_port']
    self._ssh_username = 'root'
    self._device_dirs = ChromeOSDirs('/usr/local')
    super(ChromeOSBuildStep, self).__init__(args=args, **kwargs)
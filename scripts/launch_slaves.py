import optparse
import os
if os.name == 'nt':
  import win32api
  import string
import subprocess
import sys

DEFAULT_SLAVENAME = 'production-slave'
SVN_URL = 'https://skia.googlecode.com/svn/buildbot'
DRIVE_MAPPING=True

if os.name == 'nt':
  def GetFirstFreeDriveLetter():
    all_possible = [c for c in string.uppercase]
    in_use = win32api.GetLogicalDriveStrings()
    free = list(set(all_possible) - set(in_use))
    return free[0]

def StartSlave(slavename):
  print 'Starting slave: %s' % slavename
  slave_dir = os.path.join(os.path.realpath(os.curdir),
                           slavename)
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)
    os.chdir(slave_dir)
    if os.name == 'nt':
      gclient = 'gclient.bat'
    else:
      gclient = 'gclient'
    proc = subprocess.Popen([gclient, 'config', SVN_URL])
    if proc.wait() != 0:
      raise Exception('Could not successfully configure gclient.')
    print 'configured.'
    proc = subprocess.Popen([gclient, 'sync'])
    if proc.wait() != 0:
      raise Exception('Sync failed.')

  try:
    os.remove(os.path.join(slave_dir, 'buildbot', 'third_party',
                           'chromium_buildbot', 'slave', 'twistd.pid'))
  except:
    pass

  if os.name == 'nt':
    # We run different commands for the Windows shell
    if DRIVE_MAPPING:
      drive_letter = GetFirstFreeDriveLetter()
      print 'Mapping %c' % drive_letter
      cmd = 'net use %c: \\\\localhost\%s' % (drive_letter,
                                              slave_dir.replace(':', '$'))
      print 'Running cmd: %s' % cmd
      proc = subprocess.Popen(cmd)
      if proc.wait() != 0:
        raise Exception('Could not map %c' % drive_letter)
      cmd = '%c:' % drive_letter
      os.chdir(os.path.join('%c:' % drive_letter))
      print os.path.realpath(os.curdir)
    os.chdir(os.path.join('buildbot', 'slave'))
    cmd = 'setlocal&&'
    if slavename != DEFAULT_SLAVENAME:
      cmd += 'set TESTING_SLAVENAME=%s&&' % slavename
    cmd += 'run_slave.bat'
    cmd += '&& endlocal'
  else:
    cmd = ''
    if slavename != DEFAULT_SLAVENAME:
      cmd += 'TESTING_SLAVENAME=%s ' % slavename
    cmd += 'make -C %s restart' % os.path.join(slave_dir, 'buildbot', 'slave')
  print 'Running cmd: %s' % cmd
  subprocess.Popen(cmd, shell=True)

def LoadConfigFile(config_file):
  f = open(config_file, 'r')
  slaves = []
  for line in f:
    line = line.rstrip('\r\n')
    if line != '':
      slaves.append(line)
  f.close()
  return slaves

def main():
  """ Launch local build slave instances """
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--config_file',
      help='file containing slavenames to run on this machine')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  if not options.config_file:
    raise Exception('missing command-line option %s; rerun with --help' %
        '--config_file')
  slaves = LoadConfigFile(options.config_file)
  if not slaves:
    slaves = [DEFAULT_SLAVENAME]
  print 'In dir: %s' % os.path.realpath(os.curdir);
  for slavename in slaves:
    StartSlave(slavename)

if '__main__' == __name__:
  sys.exit(main())

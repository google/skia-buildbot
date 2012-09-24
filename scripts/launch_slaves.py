import optparse
import os
import pickle
import shutil
import subprocess
import sys

DEFAULT_SLAVENAME = 'production-slave'
DEFAULT_DIR = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                            DEFAULT_SLAVENAME)

def StartSlave(slavename):
  print 'Starting slave: %s' % slavename
  slave_dir = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                           slavename)
  if not os.path.isdir(slave_dir):
    print 'creating directory: %s' % slave_dir
    shutil.copytree(DEFAULT_DIR, slave_dir)
  try:
    os.remove(os.path.join(slave_dir, 'buildbot', 'third_party', 'chromium_buildbot', 'slave', 'twistd.pid'))
  except:
    pass
  cmd = 'setlocal &&'
  cmd += 'net use O: \\\\localhost\%s&&O:&&cd %s&&' % (slave_dir.replace(':', '$'), os.path.join('buildbot', 'slave'))
  if slavename != DEFAULT_SLAVENAME:
    cmd += 'set TESTING_SLAVENAME=%s&&' % slavename
  cmd += 'run_slave.bat'
  cmd += '&& endlocal'
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

def main(argv):
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
  for slavename in slaves:
    StartSlave(slavename)

if '__main__' == __name__:
  sys.exit(main(None))

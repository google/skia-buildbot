import os
import subprocess
import sys
import time

# Initial number of seconds to wait before retrying a command.
INITIAL_BACKOFF = 2

# Number of retries when a command fails.
RETRIES = 3

# Assets located here.
ASSET_DIR = '/usr/local/share/assets'

# Directory where the dev images are.
DEV_IMG_DIR = ASSET_DIR + '/devimages'

# Path to the provisioning profile.
PROVISIONING_PROF = ASSET_DIR + '/development.mobileprovision'

# Fields that should be ignored when getting the iOS device info
IGNORE_FIELDS = set([
  'CompassCalibration'
  'ProximitySensorCalibration'
  'SoftwareBehavior'
])

def ios_get_device_ids():
  """Returns the ids of all attached devices.
     This will also work when the attached
     device is not fully booted."""
  output = _run_cmd('idevice_id --list')
  ret = []
  for line in output.splitlines():
    if line.strip():
      ret.append(line.strip())
  return ret

def ios_get_devices():
  """ Returns instances of IOSDevice for each attached device."""
  return [IOSDevice(x) for x in ios_get_device_ids()]

class IOSDevice(object):
  def __init__(self, dev_id):
    self._id = dev_id
    self._setup_device()

  def get_state(self):
    """Returns a dictionary to be used"""
    return _get_kv_pairs(_run_cmd('ideviceinfo -u %s' % self._id))

  def reboot(self):
    """Reboots the device."""
    _run_cmd('idevicediagnostics restart -u %s' % self._id)

  def _setup_device(self):
    """Does all the necessary setup to run apps on the device."""
    # Pair the device with the host.
    ret = _run_ret_value('idevicepair -u %s validate' % self._id)
    if ret != 0:
      _run_cmd('idevicepair -u %s pair' % self._id)

    # Check if a developer images has been mounted already.
    kv = _get_kv_pairs(_run_cmd('ideviceimagemounter -u %s -l' % self._id))
    if kv['ImagePresent'] != 'true':
      # Mount the developer image.
      img, imgSig = self._get_dev_image()
      cmd = 'ideviceimagemounter -u %s %s %s' % (self._id, img, imgSig)
      kv = _get_kv_pairs(_run_cmd(cmd))
      assert(kv['Status'] == 'Complete')

    # Install the provisioning profile.
    _run_cmd('ideviceprovision -u %s remove-all' % self._id)
    cmd = 'ideviceprovision -u %s install %s' % (self._id, PROVISIONING_PROF)
    _run_cmd(cmd)

  def _get_dev_image(self):
    # Get the version of the device
    state = self.get_state()
    ver = state['ProductVersion'].split('.', 2)[:2]
    prefix = '.'.join(ver)

    # iterate over the contents of the image dir and find the right image
    for dirname in os.listdir(DEV_IMG_DIR):
      path = os.path.join(DEV_IMG_DIR, dirname)
      if os.path.isdir(path) and dirname.startswith(prefix):
        contents = os.listdir(path)
        devImage = [x for x in contents if x.endswith('.dmg')][0]
        devImageSig = [x for x in contents if x.endswith('.dmg.signature')][0]
        return os.path.join(path, devImage), os.path.join(path, devImageSig)
    raise ValueError('Unable to find dev images.')

# Utility functions.
def _get_kv_pairs(output):
  """Extract key/value pairs from output."""
  ret = {}
  for line in output.splitlines():
    if not line.startswith(' '):
      parts = [x.strip() for x in line.strip().split(':', 1)]
      if (len(parts) == 2) and parts[0] not in IGNORE_FIELDS and parts[1]:
        ret[parts[0]] = parts[1]
  return ret

def _run_ret_value(cmd):
  """ Run the given command and return the exit status"""
  try:
    _run_cmd(cmd)
    return 0
  except subprocess.CalledProcessError as ex:
    return ex.returncode

def _run_cmd(cmd):
  """Run the given command and return the output. If an error
    occurs the command will be retried. If the problem persists
    an instance of CalledProcessError is thrown. """
  backoff = INITIAL_BACKOFF
  retry = 0
  while True:
    try:
      args = cmd.strip().split()
      output = subprocess.check_output(args, stderr=sys.stderr)
      return output
    except subprocess.CalledProcessError:
      retry += 1
      if retry >= RETRIES:
        raise
      time.sleep(backoff)
      backoff *= 2

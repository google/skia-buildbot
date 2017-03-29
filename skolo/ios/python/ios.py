import subprocess
import sys

# Fields that should be ignored when getting the iOS device info
IGNORE_FIELDS = set([
  "CompassCalibration"
  "ProximitySensorCalibration"
  "SoftwareBehavior"
])

def get_devices():
  """ Returns instances of IOSDevice for each attached device."""
  output = _run_cmd("idevice_id --list")
  ret = []
  for line in output.splitlines():
    if line.strip():
      ret.append(IOSDevice(line.strip()))
  return ret

class IOSDevice(object):
  def __init__(self, id):
    self._id = id

  def get_state(self):
    """Returns a dictionary to be used"""
    output = _run_cmd("ideviceinfo -u %s" % self._id)
    ret = {}
    for line in output.splitlines():
      if not line.startswith(" "):
        parts = [x.strip() for x in line.strip().split(":", 1)]
        if (len(parts) == 2) and parts[0] not in IGNORE_FIELDS and parts[1]:
          ret[parts[0]] = parts[1]
    return ret

  def reboot():
    """Reboots the device."""
    _run_cmd("idevicediagnostics restart -u %s" % self._id)

def _run_cmd(cmd):
  args = cmd.strip().split()
  output = subprocess.check_output(args, stderr=sys.stderr)
  return output


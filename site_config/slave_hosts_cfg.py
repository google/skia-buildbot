""" This file contains configuration information for the build slave host
machines. """


import collections
import json
import ntpath
import os
import posixpath
import sys


CHROMECOMPUTE_BUILDBOT_PATH = ['storage', 'skia-repo', 'buildbot']

# Indicates that this machine is not connected to a KVM switch.
NO_KVM_SWITCH = '(not on KVM)'
NO_KVM_NUM = '(not on KVM)'

# Indicates that this machine has no static IP address.
NO_IP_ADDR = '(no static IP)'

# Files to copy into buildslave checkouts.
CHROMEBUILD_COPIES = [
  {
    "source": ".bot_password",
    "destination": "build/site_config",
  },
]

KVM_SWITCH_DOOR = 'DOOR'   # KVM switch closest to the door.
KVM_SWITCH_OFFICE = 'OFFICE' # KVM switch closest to the office.

LAUNCH_SCRIPT_UNIX = ['scripts', 'skiabot-slave-start-on-boot.sh']
LAUNCH_SCRIPT_WIN = ['scripts', 'skiabot-slave-start-on-boot.bat']


# Data for all Skia build slave hosts.
_slave_host_dicts = {

################################ Linux Machines ################################

  'skiabot-shuttle-ubuntu12-gtx660-001': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-gtx660-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.113',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'E',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu12-gtx660-002': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-gtx660-bench', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.122',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'F',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu15-000': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-nexusplayer-001', '0',  False),
      ('skiabot-shuttle-ubuntu12-nexusplayer-002', '1',  False),
      ('skiabot-shuttle-ubuntu12-nexus10-001',     '2', False),
      ('skiabot-shuttle-ubuntu12-nexus10-003',     '3', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.112',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'A',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu15-001': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-galaxys3-001',    '0', False),
      ('skiabot-shuttle-ubuntu12-nexus9-001',      '2', False),
      ('skiabot-shuttle-ubuntu12-nexus9-002',      '3', False),
      ('skiabot-shuttle-ubuntu12-nexus9-003',      '4', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.109',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'B',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu15-003': {
    'slaves': [
      ('skiabot-shuttle-ubuntu15-androidone-001',  '7', False),
      ('skiabot-shuttle-ubuntu15-androidone-002',  '8', False),
      ('skiabot-shuttle-ubuntu15-androidone-003',  '9', False),
      ('skiabot-shuttle-ubuntu15-nexus6-001',      '4', False),
      ('skiabot-shuttle-ubuntu15-nexus6-002',      '5', False),
      ('skiabot-shuttle-ubuntu15-nexus6-003',      '6', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.138',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'D',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu13-xxx': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.120',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'H',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-005': {
    'slaves': [
      ('skiabot-linux-swarm-048', '48', False),
      ('skiabot-linux-swarm-049', '49', False),
      ('skiabot-linux-swarm-050', '50', False),
      #('skiabot-linux-swarm-051', '51', False),
      #('skiabot-linux-swarm-052', '52', False),
      #('skiabot-linux-swarm-053', '53', False),
      #('skiabot-linux-swarm-054', '54', False),
      #('skiabot-linux-swarm-055', '55', False),
      #('skiabot-linux-swarm-056', '56', False),
      #('skiabot-linux-swarm-057', '57', False),
      #('skiabot-linux-swarm-058', '58', False),
      #('skiabot-linux-swarm-059', '59', False),
      #('skiabot-linux-swarm-060', '60', False),
      #('skiabot-linux-swarm-061', '61', False),
      #('skiabot-linux-swarm-062', '62', False),
      #('skiabot-linux-swarm-063', '63', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-006': {
    'slaves': [
      ('skiabot-linux-swarm-032', '32', False),
      ('skiabot-linux-swarm-033', '33', False),
      ('skiabot-linux-swarm-034', '34', False),
      ('skiabot-linux-swarm-035', '35', False),
      ('skiabot-linux-swarm-036', '36', False),
      ('skiabot-linux-swarm-037', '37', False),
      ('skiabot-linux-swarm-038', '38', False),
      ('skiabot-linux-swarm-039', '39', False),
      ('skiabot-linux-swarm-040', '40', False),
      ('skiabot-linux-swarm-041', '41', False),
      ('skiabot-linux-swarm-042', '42', False),
      ('skiabot-linux-swarm-043', '43', False),
      ('skiabot-linux-swarm-044', '44', False),
      ('skiabot-linux-swarm-045', '45', False),
      ('skiabot-linux-swarm-046', '46', False),
      ('skiabot-linux-swarm-047', '47', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-007': {
    'slaves': [
      ('skiabot-linux-swarm-016', '16', False),
      ('skiabot-linux-swarm-017', '17', False),
      ('skiabot-linux-swarm-018', '18', False),
      ('skiabot-linux-swarm-019', '19', False),
      ('skiabot-linux-swarm-020', '20', False),
      ('skiabot-linux-swarm-021', '21', False),
      ('skiabot-linux-swarm-022', '22', False),
      ('skiabot-linux-swarm-023', '23', False),
      ('skiabot-linux-swarm-024', '24', False),
      ('skiabot-linux-swarm-025', '25', False),
      ('skiabot-linux-swarm-026', '26', False),
      ('skiabot-linux-swarm-027', '27', False),
      ('skiabot-linux-swarm-028', '28', False),
      ('skiabot-linux-swarm-029', '29', False),
      ('skiabot-linux-swarm-030', '30', False),
      ('skiabot-linux-swarm-031', '31', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-008': {
    'slaves': [
      ('skiabot-linux-swarm-000', '0', False),
      ('skiabot-linux-swarm-001', '1', False),
      ('skiabot-linux-swarm-002', '2', False),
      ('skiabot-linux-swarm-003', '3', False),
      ('skiabot-linux-swarm-004', '4', False),
      ('skiabot-linux-swarm-005', '5', False),
      ('skiabot-linux-swarm-006', '6', False),
      ('skiabot-linux-swarm-007', '7', False),
      ('skiabot-linux-swarm-008', '8', False),
      ('skiabot-linux-swarm-009', '9', False),
      ('skiabot-linux-swarm-010', '10', False),
      ('skiabot-linux-swarm-011', '11', False),
      ('skiabot-linux-swarm-012', '12', False),
      ('skiabot-linux-swarm-013', '13', False),
      ('skiabot-linux-swarm-014', '14', False),
      ('skiabot-linux-swarm-015', '15', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-009': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-010': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-011': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-012': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-013': {
    'slaves': [
      ('skiabot-ct-dm-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-014': {
    'slaves': [
      ('skia-android-canary', '0', True),
      ('skia-android-build-000', '1', True),
      ('skia-android-build-001', '2', True),
      ('skia-android-build-002', '3', True),
      ('skia-android-build-003', '4', True),
      ('skia-android-build-004', '5', True),
      ('skia-android-build-005', '6', True),
      ('skia-android-build-006', '7', True),
      ('skia-android-build-007', '8', True),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-015': {
    'slaves': [
      ('skiabot-linux-housekeeper-001', '0', False),
      ('skiabot-linux-housekeeper-003', '2', False),
      ('skiabot-ct-trybot-000', '3', False),
      ('skiabot-ct-trybot-001', '4', False),
      ('skiabot-ct-dm-000', '5', False),
      ('skiabot-ct-dm-002', '6', False),
      ('skiabot-ct-dm-003', '7', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-024': {
    'slaves': [
      ('skiabot-linux-infra-000', '0', False),
      ('skiabot-linux-infra-001', '1', False),
      ('skiabot-linux-infra-002', '2', False),
      ('skiabot-linux-infra-003', '3', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-101': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

################################# Mac Machines #################################

  'skiabot-mac-10_10-ios': {
    'slaves': [
      ('skiabot-ipad4-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.141',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': '2',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-mac-10_10-001': {
    'slaves': [
      ('skiabot-shuttle-ubuntu15-nvidia-shield-001', '1', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.121',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': '5',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

############################### Windows Machines ###############################

  'win8-4790k-001': {
    'slaves': [
      ('skiabot-shuttle-win8-i7-4790k-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.129',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': '1',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'win8-4790k-002': {
    'slaves': [
      ('skiabot-shuttle-win8-i7-4790k-002', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.136',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': '3',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'win10-gtx660-00': {
    'slaves': [
      ('skiabot-shuttle-win10-gtx660-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.133',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'B',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'win8-gtx960-002': {
    'slaves': [
      ('skiabot-shuttle-win8-gtx960-002', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.142',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': '2',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

############################ Machines in Chrome Golo ###########################

  'build3-a3': {
    'slaves': [
      ('build3-a3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'build4-a3': {
    'slaves': [
      ('build4-a3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'vm255-m3': {
    'slaves': [
      ('vm255-m3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'slave11-c3': {
    'slaves': [
      ('slave11-c3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

}


# Class which holds configuration data describing a build slave host.
SlaveHostConfig = collections.namedtuple('SlaveHostConfig',
                                         ('hostname, slaves, copies,'
                                          ' ip, kvm_switch, kvm_num,'
                                          ' path_to_buildbot,'
                                          ' launch_script'))


SLAVE_HOSTS = {}
for (_hostname, _config) in _slave_host_dicts.iteritems():
  SLAVE_HOSTS[_hostname] = SlaveHostConfig(hostname=_hostname,
                                           **_config)


def default_slave_host_config(hostname):
  """Return a default configuration for the given hostname.

  Assumes that the slave host is the machine on which this function is called.

  Args:
      hostname: string; name of the build slave host.
  Returns:
      SlaveHostConfig instance with configuration for this machine.
  """
  path_to_buildbot = os.path.join(os.path.dirname(__file__), os.pardir)
  path_to_buildbot = os.path.abspath(path_to_buildbot).split(os.path.sep)
  launch_script = LAUNCH_SCRIPT_WIN if os.name == 'nt' else LAUNCH_SCRIPT_UNIX
  return SlaveHostConfig(
    hostname=hostname,
    slaves=[(hostname, '0', True)],
    copies=CHROMEBUILD_COPIES,
    ip=None,
    kvm_switch=None,
    kvm_num=None,
    path_to_buildbot=path_to_buildbot,
    launch_script=launch_script,
  )


def get_slave_host_config(hostname):
  """Helper function for retrieving slave host configuration information.

  If the given hostname is unknown, returns a default config.

  Args:
      hostname: string; the hostname of the slave host machine.
  Returns:
      SlaveHostConfig instance representing the given host.
  """
  return SLAVE_HOSTS.get(hostname, default_slave_host_config(hostname))


if __name__ == '__main__':
  print json.dumps(_slave_host_dicts)

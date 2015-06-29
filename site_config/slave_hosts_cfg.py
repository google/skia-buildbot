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
    "source": ".boto",
    "destination": "build/site_config",
  },
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

  'skiabot-shuttle-ubuntu12-gtx550ti-001': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-gtx550ti-001', '0', False),
      ('skiabot-shuttle-ubuntu12-003',          '1', False),
      ('skiabot-shuttle-ubuntu12-004',          '2', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.132',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'E',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

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

  'skiabot-shuttle-ubuntu15-002': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-nexus5-001',      '0',  False),
      ('skiabot-shuttle-ubuntu12-nexus5-002',      '1',  False),
      ('skiabot-shuttle-ubuntu12-nexus7-001',      '2', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.110',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'C',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-shuttle-ubuntu15-003': {
    'slaves': [
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

  'skiabot-shuttle-ubuntu15-004': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-nexus7-002', '1', False),
      ('skiabot-shuttle-ubuntu12-nexus7-003', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.139',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': 'F',
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

  'skia-vm-001': {
    'slaves': [
      ('skiabot-linux-compile-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-002': {
    'slaves': [
      ('skiabot-linux-compile-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-003': {
    'slaves': [
      ('skiabot-linux-compile-002', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-004': {
    'slaves': [
      ('skiabot-linux-compile-003', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-005': {
    'slaves': [
      ('skiabot-linux-compile-004', '0', False),
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
      ('skiabot-linux-compile-005', '0', False),
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
      ('skiabot-linux-compile-006', '0', False),
      ('skiabot-linux-housekeeper-000', '1', False),
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
      ('skiabot-linux-housekeeper-002', '1', False),
      ('skiabot-linux-housekeeper-003', '2', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-016': {
    'slaves': [
      ('skiabot-win-compile-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-017': {
    'slaves': [
      ('skiabot-win-compile-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-018': {
    'slaves': [
      ('skiabot-win-compile-002', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-019': {
    'slaves': [
      ('skiabot-win-compile-003', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-020': {
    'slaves': [
      ('skiabot-linux-tester-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-021': {
    'slaves': [
      ('skiabot-linux-tester-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-022': {
    'slaves': [
      ('skiabot-linux-tester-002', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-023': {
    'slaves': [
      ('skiabot-linux-tester-003', '0', False),
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
      ('skia-vm-024', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-025': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-026': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-027': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-028': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-029': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': CHROMECOMPUTE_BUILDBOT_PATH,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skia-vm-030': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-031': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'skia-vm-032': {
    'slaves': [
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
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

  'skiabot-macmini-10_8-001': {
    'slaves': [
      ('skiabot-macmini-10_8-000', '0', False),
      ('skiabot-macmini-10_8-001', '1', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.125',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': '8',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-macmini-10_8-002': {
    'slaves': [
      ('skiabot-macmini-10_8-bench', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.135',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': '6',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'skiabot-mac-10_8-compile': {
    'slaves': [
      ('skiabot-mac-10_8-compile-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.104',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': '7',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

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
      ('skiabot-shuttle-ubuntu12-galaxys4-001',      '2', False),
      ('skiabot-shuttle-ubuntu12-galaxys4-002',      '3', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.121',
    'kvm_switch': KVM_SWITCH_OFFICE,
    'kvm_num': '5',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

############################### Windows Machines ###############################

  'win8-4790k-000': {
    'slaves': [
      ('skiabot-shuttle-win8-i7-4790k-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.128',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'H',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

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

  'win8-gtx660-001': {
    'slaves': [
      ('skiabot-shuttle-win8-gtx660-bench', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.133',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'B',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'win8-gtx960-001': {
    'slaves': [
      ('skiabot-shuttle-win8-gtx960-001', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.108',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'G',
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

  'win8-hd7770-000': {
    'slaves': [
      ('skiabot-shuttle-win8-hd7770-000', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.117',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'C',
    'path_to_buildbot': ['buildbot'],
    'launch_script': LAUNCH_SCRIPT_WIN,
  },

  'win8-hd7770-001': {
    'slaves': [
      ('skiabot-shuttle-win8-hd7770-bench', '0', False),
    ],
    'copies': CHROMEBUILD_COPIES,
    'ip': '192.168.1.107',
    'kvm_switch': KVM_SWITCH_DOOR,
    'kvm_num': 'D',
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

  'build5-a3': {
    'slaves': [
      ('build5-a3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'build5-m3': {
    'slaves': [
      ('build5-m3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'build32-a3': {
    'slaves': [
      ('build32-a3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'build33-a3': {
    'slaves': [
      ('build33-a3', '0', False),
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

  'vm690-m3': {
    'slaves': [
      ('vm690-m3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'vm691-m3': {
    'slaves': [
      ('vm691-m3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'vm692-m3': {
    'slaves': [
      ('vm692-m3', '0', False),
    ],
    'copies': None,
    'ip': NO_IP_ADDR,
    'kvm_switch': NO_KVM_SWITCH,
    'kvm_num': NO_KVM_NUM,
    'path_to_buildbot': None,
    'launch_script': LAUNCH_SCRIPT_UNIX,
  },

  'vm693-m3': {
    'slaves': [
      ('vm693-m3', '0', False),
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

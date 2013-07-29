""" This file contains configuration information for the build slave host
machines. """


# Files to copy into buildslave checkouts.
_DEFAULT_COPIES = [
  {
    "source": ".boto",
    "destination": "buildbot/third_party/chromium_buildbot/site_config",
  },
  {
    "source": ".autogen_svn_username",
    "destination": "buildbot/site_config",
  },
  {
    "source": ".autogen_svn_password",
    "destination": "buildbot/site_config",
  },
]


SLAVE_HOSTS = {

################################ Linux Machines ################################

  'skiabot-shuttle-ubuntu12-ati5770-001': {
    'slaves': [
      'skiabot-shuttle-ubuntu12-ati5770-001',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.132',
    'kvm_num': '2',
  },

  'skiabot-shuttle-ubuntu12-android-003': {
    'slaves': [
      'skiabot-shuttle-ubuntu12-nexuss-001',
      'skiabot-shuttle-ubuntu12-nexuss-002',
      'skiabot-shuttle-ubuntu12-xoom-001',
      'skiabot-shuttle-ubuntu12-xoom-002',
      'skiabot-shuttle-ubuntu12-xoom-003',
      'skiabot-shuttle-ubuntu12-galaxynexus-001',
      'skiabot-shuttle-ubuntu12-nexus4-001',
      'skiabot-shuttle-ubuntu12-nexus7-001',
      'skiabot-shuttle-ubuntu12-nexus7-002',
      'skiabot-shuttle-ubuntu12-nexus7-003',
      'skiabot-shuttle-ubuntu12-nexus10-001',
      'skiabot-shuttle-ubuntu12-nexus10-003',
      'skiabot-shuttle-ubuntu12-razri-001',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.110',
    'kvm_num': '8',
  },

  'skiabot-shuttle-ubuntu12-xxx': {
    'slaves': [
      'skiabot-shuttle-ubuntu12-000',
      'skiabot-shuttle-ubuntu12-001',
      'skiabot-shuttle-ubuntu12-002',
      'skiabot-shuttle-ubuntu12-003',
      'skiabot-shuttle-ubuntu12-004',
      'skiabot-shuttle-ubuntu12-005',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.108',
    'kvm_num': '7',
  },

  'skia-compile1-a': {
    'slaves': [
      'skiabot-linux-compile-vm-a-000',
      'skiabot-linux-compile-vm-a-001',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile2-a': {
    'slaves': [
      'skiabot-linux-compile-vm-a-002',
      'skiabot-linux-compile-vm-a-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile3-a': {
    'slaves': [
      'skiabot-linux-compile-vm-a-004',
      'skiabot-linux-compile-vm-a-005',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile4-a': {
    'slaves': [
      'skiabot-linux-compile-vm-a-006',
      'skiabot-linux-compile-vm-a-007',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile5-a': {
    'slaves': [
      'skiabot-linux-compile-vm-a-008',
      'skiabot-linux-compile-vm-a-009',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-housekeeping-slave-a': {
    'slaves': [
      'skia-housekeeping-slave-a',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile1-b': {
    'slaves': [
      'skiabot-linux-compile-vm-b-000',
      'skiabot-linux-compile-vm-b-001',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile2-b': {
    'slaves': [
      'skiabot-linux-compile-vm-b-002',
      'skiabot-linux-compile-vm-b-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile3-b': {
    'slaves': [
      'skiabot-linux-compile-vm-b-004',
      'skiabot-linux-compile-vm-b-005',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile4-b': {
    'slaves': [
      'skiabot-linux-compile-vm-b-006',
      'skiabot-linux-compile-vm-b-007',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-compile5-b': {
    'slaves': [
      'skiabot-linux-compile-vm-b-008',
      'skiabot-linux-compile-vm-b-009',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

  'skia-housekeeping-slave-b': {
    'slaves': [
      'skia-housekeeping-slave-b',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

################################# Mac Machines #################################

  'skiabot-macmini-10_6-001': {
    'slaves': [
      'skiabot-macmini-10_6-000',
      'skiabot-macmini-10_6-001',
      'skiabot-macmini-10_6-002',
      'skiabot-macmini-10_6-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.144',
    'kvm_num': 'A',
  },

  'skiabot-macmini-10_6-002': {
    'slaves': [
      'skiabot-macmini-10_6-bench',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.121',
    'kvm_num': 'D',
  },

  'skiabot-macmini-10_7-001': {
    'slaves': [
      'skiabot-macmini-10_7-000',
      'skiabot-macmini-10_7-001',
      'skiabot-macmini-10_7-002',
      'skiabot-macmini-10_7-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.137',
    'kvm_num': 'B',
  },

  'skiabot-macmini-10_7-002': {
    'slaves': [
      'skiabot-macmini-10_7-bench',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.124',
    'kvm_num': 'C',
  },

  'skiabot-macmini-10_8-001': {
    'slaves': [
      'skiabot-macmini-10_8-000',
      'skiabot-macmini-10_8-001',
      'skiabot-macmini-10_8-002',
      'skiabot-macmini-10_8-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.140',
    'kvm_num': 'F',
  },

  'skiabot-macmini-10_8-002': {
    'slaves': [
      'skiabot-macmini-10_8-bench',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.135',
    'kvm_num': 'G',
  },

  'skiabot-mac-10_6-compile': {
    'slaves': [
      'skiabot-mac-10_6-compile-000',
      'skiabot-mac-10_6-compile-001',
      'skiabot-mac-10_6-compile-002',
      'skiabot-mac-10_6-compile-003',
      'skiabot-mac-10_6-compile-004',
      'skiabot-mac-10_6-compile-005',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.111',
    'kvm_num': 'N/A',
  },

  'skiabot-mac-10_7-compile': {
    'slaves': [
      'skiabot-mac-10_7-compile-000',
      'skiabot-mac-10_7-compile-001',
      'skiabot-mac-10_7-compile-002',
      'skiabot-mac-10_7-compile-003',
      'skiabot-mac-10_7-compile-004',
      'skiabot-mac-10_7-compile-005',
      'skiabot-mac-10_7-compile-006',
      'skiabot-mac-10_7-compile-007',
      'skiabot-mac-10_7-compile-008',
      'skiabot-mac-10_7-compile-009',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.120',
    'kvm_num': 'N/A',
  },

  'borenet-mac.cnc.corp.google.com': {
    'slaves': [
      'skiabot-mac-10_8-compile-000',
      'skiabot-mac-10_8-compile-001',
      'skiabot-mac-10_8-compile-002',
      'skiabot-mac-10_8-compile-003',
      'skiabot-mac-10_8-compile-004',
      'skiabot-mac-10_8-compile-005',
      'skiabot-mac-10_8-compile-006',
      'skiabot-mac-10_8-compile-007',
      'skiabot-mac-10_8-compile-008',
      'skiabot-mac-10_8-compile-009',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': 'N/A',
    'kvm_num': 'N/A',
  },

############################### Windows Machines ###############################

  'win7-intel-002': {
    'slaves': [
      'skiabot-shuttle-win7-intel-bench',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.139',
    'kvm_num': '3',
  },

  'win7-intel-003': {
    'slaves': [
      'skiabot-shuttle-win7-intel-000',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.114',
    'kvm_num': '4',
  },

  'win7-intel-004': {
    'slaves': [
      'skiabot-shuttle-win7-intel-special-000',
      'skiabot-shuttle-win7-intel-special-001',
      'skiabot-shuttle-win7-intel-special-002',
      'skiabot-shuttle-win7-intel-special-003',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.119',
    'kvm_num': '6',
  },

  'win7-compile1': {
    'slaves': [
      'skiabot-win-compile-000',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.101',
    'kvm_num': 'N/A',
  },

  'win7-compile2': {
    'slaves': [
      'skiabot-win-compile-004',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.113',
    'kvm_num': 'N/A',
  },

  'win8compile000': {
    'slaves': [
      'skiabot-win8-compile-000',
    ],
    'copies': _DEFAULT_COPIES,
    'ip': '192.168.1.117',
    'kvm_num': 'N/A',
  },
}


def GetSlaveHostConfig(hostname):
  """ Helper function for retrieving configuration information for a given slave
  host machine. If no configuration exists for the given hostname, return a
  default.

  hostname: string; the hostname of the slave host machine.
  """
  default_cfg = {
    'slaves': [hostname],
    'copies': _DEFAULT_COPIES,
  }
  return SLAVE_HOSTS.get(hostname, default_cfg)

""" This file contains configuration information for the build slave host
machines. """


import collections
import ntpath
import os
import posixpath

import skia_vars


# Indicates that this machine is not connected to a KVM switch.
NO_KVM_NUM = '(not on KVM)'

# Indicates that this machine has no static IP address.
NO_IP_ADDR = '(no static IP)'

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

GCE_CLUSTER = skia_vars.GetGlobalVariable('gce_cluster')
GCE_PROJECT = skia_vars.GetGlobalVariable('gce_project')
GCE_USERNAME = skia_vars.GetGlobalVariable('gce_username')

SKIALAB_ROUTER_IP = skia_vars.GetGlobalVariable('skialab_router_ip')
SKIALAB_USERNAME = skia_vars.GetGlobalVariable('skialab_username')


# Procedures for logging in to the host machines.

def skia_lab_login(hostname, config):
  """Procedure for logging into SkiaLab machines."""
  # The multiple "-t"s are needed in order to force tty allocation, which gives
  # us the expected interactive behavior, even with two levels of SSH.
  return [
    'ssh', '-t', '-t', '%s@%s' % (SKIALAB_USERNAME, SKIALAB_ROUTER_IP),
    'ssh', '%s@%s' % (SKIALAB_USERNAME, config['ip'])
  ]


def compute_engine_login(hostname, config):
  """Procedure for logging into Skia GCE instances."""
  return [
    'gcutil', '--project=%s' % GCE_PROJECT,
    'ssh', '--ssh_user=%s' % GCE_USERNAME, hostname,
  ]


# Data for all Skia build slave hosts.
_slave_host_dicts = {

################################ Linux Machines ################################

  'skiabot-shuttle-ubuntu12-ati5770-001': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-ati5770-001', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.132',
    'kvm_num': 'A',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-shuttle-ubuntu12-android-003': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-nexuss-001', '0'),
      ('skiabot-shuttle-ubuntu12-nexuss-002', '1'),
      ('skiabot-shuttle-ubuntu12-xoom-001', '3'),
      ('skiabot-shuttle-ubuntu12-xoom-003', '5'),
      ('skiabot-shuttle-ubuntu12-galaxynexus-001', '6'),
      ('skiabot-shuttle-ubuntu12-nexus4-001', '7'),
      ('skiabot-shuttle-ubuntu12-nexus7-001', '8'),
      ('skiabot-shuttle-ubuntu12-nexus7-002', '9'),
      ('skiabot-shuttle-ubuntu12-nexus7-003', '10'),
      ('skiabot-shuttle-ubuntu12-nexus10-001', '11'),
      ('skiabot-shuttle-ubuntu12-nexus10-003', '12'),
      ('skiabot-shuttle-ubuntu12-intel-rhb-001', '13'),
      ('skiabot-shuttle-ubuntu12-logan-001', '13'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.110',
    'kvm_num': 'C',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-shuttle-ubuntu12-xxx': {
    'slaves': [
      ('skiabot-shuttle-ubuntu12-001', '1'),
      ('skiabot-shuttle-ubuntu12-002', '2'),
      ('skiabot-shuttle-ubuntu12-003', '3'),
      ('skiabot-shuttle-ubuntu12-004', '4'),
      ('skiabot-shuttle-ubuntu12-005', '5'),
      ('skiabot-shuttle-ubuntu12-006', '6'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.109',
    'kvm_num': 'B',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-shuttle-ubuntu13-xxx': {
    'slaves': [
      ('skiabot-shuttle-ubuntu13-000', '0'),
      ('skiabot-shuttle-ubuntu13-001', '1'),
      ('skiabot-shuttle-ubuntu13-002', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.120',
    'kvm_num': 'D',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skia-recreate-skps': {
    'slaves': [
      ('skia-recreate-skps', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile1-a': {
    'slaves': [
      ('skiabot-linux-compile-vm-a-000', '0'),
      ('skiabot-linux-compile-vm-a-001', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile2-a': {
    'slaves': [
      ('skiabot-linux-compile-vm-a-002', '0'),
      ('skiabot-linux-compile-vm-a-003', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile3-a': {
    'slaves': [
      ('skiabot-linux-compile-vm-a-004', '0'),
      ('skiabot-linux-compile-vm-a-005', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile4-a': {
    'slaves': [
      ('skiabot-linux-compile-vm-a-006', '0'),
      ('skiabot-linux-compile-vm-a-007', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile5-a': {
    'slaves': [
      ('skiabot-linux-compile-vm-a-008', '0'),
      ('skiabot-linux-compile-vm-a-009', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-housekeeping-slave-a': {
    'slaves': [
      ('skia-housekeeping-slave-a', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile1-b': {
    'slaves': [
      ('skiabot-linux-compile-vm-b-000', '0'),
      ('skiabot-linux-compile-vm-b-001', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile2-b': {
    'slaves': [
      ('skiabot-linux-compile-vm-b-002', '0'),
      ('skiabot-linux-compile-vm-b-003', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile3-b': {
    'slaves': [
      ('skiabot-linux-compile-vm-b-004', '0'),
      ('skiabot-linux-compile-vm-b-005', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile4-b': {
    'slaves': [
      ('skiabot-linux-compile-vm-b-006', '0'),
      ('skiabot-linux-compile-vm-b-007', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-compile5-b': {
    'slaves': [
      ('skiabot-linux-compile-vm-b-008', '0'),
      ('skiabot-linux-compile-vm-b-009', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

  'skia-housekeeping-slave-b': {
    'slaves': [
      ('skia-housekeeping-slave-b', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': compute_engine_login,
    'ip': NO_IP_ADDR,
    'kvm_num': NO_KVM_NUM,
    'path_module': posixpath,
    'path_to_buildbot': ['skia-repo', 'buildbot'],
  },

################################# Mac Machines #################################

  'skiabot-macmini-10_6-001': {
    'slaves': [
      ('skiabot-macmini-10_6-000', '0'),
      ('skiabot-macmini-10_6-001', '1'),
      ('skiabot-macmini-10_6-002', '2'),
      ('skiabot-macmini-10_6-003', '3'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.144',
    'kvm_num': '2',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-macmini-10_6-002': {
    'slaves': [
      ('skiabot-macmini-10_6-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.121',
    'kvm_num': '1',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-macmini-10_7-001': {
    'slaves': [
      ('skiabot-macmini-10_7-000', '0'),
      ('skiabot-macmini-10_7-001', '1'),
      ('skiabot-macmini-10_7-002', '2'),
      ('skiabot-macmini-10_7-003', '3'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.137',
    'kvm_num': '3',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-macmini-10_7-002': {
    'slaves': [
      ('skiabot-macmini-10_7-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.124',
    'kvm_num': '4',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-macmini-10_8-001': {
    'slaves': [
      ('skiabot-macmini-10_8-000', '0'),
      ('skiabot-macmini-10_8-001', '1'),
      ('skiabot-macmini-10_8-002', '2'),
      ('skiabot-macmini-10_8-003', '3'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.141',
    'kvm_num': '8',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-macmini-10_8-002': {
    'slaves': [
      ('skiabot-macmini-10_8-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.135',
    'kvm_num': '6',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-mac-10_6-compile': {
    'slaves': [
      ('skiabot-mac-10_6-compile-000', '0'),
      ('skiabot-mac-10_6-compile-001', '1'),
      ('skiabot-mac-10_6-compile-002', '2'),
      ('skiabot-mac-10_6-compile-003', '3'),
      ('skiabot-mac-10_6-compile-004', '4'),
      ('skiabot-mac-10_6-compile-005', '5'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.111',
    'kvm_num': '8',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-mac-10_7-compile': {
    'slaves': [
      ('skiabot-mac-10_7-compile-000', '0'),
      ('skiabot-mac-10_7-compile-001', '1'),
      ('skiabot-mac-10_7-compile-002', '2'),
      ('skiabot-mac-10_7-compile-003', '3'),
      ('skiabot-mac-10_7-compile-004', '4'),
      ('skiabot-mac-10_7-compile-005', '5'),
      ('skiabot-mac-10_7-compile-006', '6'),
      ('skiabot-mac-10_7-compile-007', '7'),
      ('skiabot-mac-10_7-compile-008', '8'),
      ('skiabot-mac-10_7-compile-009', '9'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.118',
    'kvm_num': '5',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

  'skiabot-mac-10_8-compile': {
    'slaves': [
      ('skiabot-mac-10_8-compile-000', '0'),
      ('skiabot-mac-10_8-compile-001', '1'),
      ('skiabot-mac-10_8-compile-002', '2'),
      ('skiabot-mac-10_8-compile-003', '3'),
      ('skiabot-mac-10_8-compile-004', '4'),
      ('skiabot-mac-10_8-compile-005', '5'),
      ('skiabot-mac-10_8-compile-006', '6'),
      ('skiabot-mac-10_8-compile-007', '7'),
      ('skiabot-mac-10_8-compile-008', '8'),
      ('skiabot-mac-10_8-compile-009', '9'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': skia_lab_login,
    'ip': '192.168.1.104',
    'kvm_num': '7',
    'path_module': posixpath,
    'path_to_buildbot': ['buildbot'],
  },

############################### Windows Machines ###############################

  'win7-intel-002': {
    'slaves': [
      ('skiabot-shuttle-win7-intel-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.139',
    'kvm_num': 'F',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },

  'win7-intel-003': {
    'slaves': [
      ('skiabot-shuttle-win7-intel-000', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.114',
    'kvm_num': 'G',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },

  'win7-intel-004': {
    'slaves': [
      ('skiabot-shuttle-win7-intel-special-000', '0'),
      ('skiabot-shuttle-win7-intel-special-001', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.119',
    'kvm_num': 'H',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },

  'win7-compile1': {
    'slaves': [
      ('skiabot-win-compile-000', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.100',
    'kvm_num': '3',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },

  'win7-compile2': {
    'slaves': [
      ('skiabot-win-compile-004', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.112',
    'kvm_num': '2',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },
  'win8-gtx660-000': {
    'slaves': [
      ('skiabot-shuttle-win8-gtx660-000', '0'),
      ('skiabot-win8-compile-000', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.108',
    'kvm_num': 'A',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },
  'win8-gtx660-001': {
    'slaves': [
      ('skiabot-shuttle-win8-gtx660-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.133',
    'kvm_num': 'B',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },
  'win8-hd7770-000': {
    'slaves': [
      ('skiabot-shuttle-win8-hd7770-000', '0'),
      ('skiabot-win8-compile-001', '1'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.117',
    'kvm_num': 'C',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },
  'win8-hd7770-001': {
    'slaves': [
      ('skiabot-shuttle-win8-hd7770-bench', '0'),
    ],
    'copies': _DEFAULT_COPIES,
    'login_cmd': None,
    'ip': '192.168.1.107',
    'kvm_num': 'D',
    'path_module': ntpath,
    'path_to_buildbot': ['buildbot'],
  },
}


# Class which holds configuration data describing a build slave host.
SlaveHostConfig = collections.namedtuple('SlaveHostConfig',
                                         ('hostname, slaves, copies, login_cmd,'
                                          ' ip, kvm_num, path_module,'
                                          ' path_to_buildbot'))


SLAVE_HOSTS = {}
for (_hostname, _config) in _slave_host_dicts.iteritems():
  login_cmd = _config.pop('login_cmd')
  if login_cmd:
    resolved_login_cmd = login_cmd(_hostname, _config)
  else:
    resolved_login_cmd = None
  SLAVE_HOSTS[_hostname] = SlaveHostConfig(hostname=_hostname,
                                           login_cmd=resolved_login_cmd,
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
  return SlaveHostConfig(
    hostname=hostname,
    slaves=[(hostname, '0')],
    copies=_DEFAULT_COPIES,
    login_cmd=None,
    ip=None,
    kvm_num=None,
    path_module=os.path,
    path_to_buildbot=path_to_buildbot
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

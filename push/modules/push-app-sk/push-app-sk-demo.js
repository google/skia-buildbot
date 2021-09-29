import './index.js';

import { $$ } from 'common-sk/modules/dom';

const ele = $$('#ele');
const state = {
  servers: [
    {
      Name: 'afdo-chromium-autoroll',
      Installed: [
        'git-cookie-authdaemon/git-cookie-authdaemon:rmistry@rmistry1.cnc.corp.google.com:2018-01-24T19:44:12Z:58e6b9f3e99e157fa049b41686ea9e78d82dc091.deb',
        'pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-08T18:53:57Z:aad8e234d9478729c1e073dde2c6a95f5976033d.deb',
      ],
    },
    {
      Name: 'android-compile',
      Installed: [
        'git-cookie-authdaemon/git-cookie-authdaemon:rmistry@rmistry1.cnc.corp.google.com:2018-01-24T19:23:13Z:d6055d655ac587b4be50a5e15a91756b844432ca.deb',
        'pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-08T18:53:57Z:aad8e234d9478729c1e073dde2c6a95f5976033d.deb',
      ],
    },
  ],
  packages: {
    'git-cookie-authdaemon': [
      {
        Name: 'git-cookie-authdaemon/git-cookie-authdaemon:rmistry@rmistry1.cnc.corp.google.com:2018-01-24T19:44:12Z:58e6b9f3e99e157fa049b41686ea9e78d82dc091.deb',
        Hash: '58e6b9f3e99e157fa049b41686ea9e78d82dc091',
        UserID: 'rmistry@rmistry1.cnc.corp.google.com',
        Built: '2018-01-24T19:44:12Z',
        Dirty: true,
        Note: 'First committed push of authdaemon',
        Services: [
          'git-cookie-authdaemon.service',
        ],
      },
      {
        Name: 'git-cookie-authdaemon/git-cookie-authdaemon:rmistry@rmistry1.cnc.corp.google.com:2018-01-24T19:23:13Z:d6055d655ac587b4be50a5e15a91756b844432ca.deb',
        Hash: 'd6055d655ac587b4be50a5e15a91756b844432ca',
        UserID: 'rmistry@rmistry1.cnc.corp.google.com',
        Built: '2018-01-24T19:23:13Z',
        Dirty: true,
        Note: 'First push of authdaemon',
        Services: [
          'git-cookie-authdaemon.service',
        ],
      },
      {
        Name: 'git-cookie-authdaemon/git-cookie-authdaemon:borenet@borenet0.cnc.corp.google.com:2017-12-14T15:41:16Z:b1685a91bf8de67f8a475574c306c00f6d30c3fc.deb',
        Hash: 'b1685a91bf8de67f8a475574c306c00f6d30c3fc',
        UserID: 'borenet@borenet0.cnc.corp.google.com',
        Built: '2017-12-14T15:41:16Z',
        Dirty: true,
        Note: '',
        Services: [
          'git-cookie-authdaemon.service',
        ],
      },
    ],
    pulld: [
      {
        Name: 'pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-08T18:53:57Z:aad8e234d9478729c1e073dde2c6a95f5976033d.deb',
        Hash: 'aad8e234d9478729c1e073dde2c6a95f5976033d',
        UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
        Built: '2018-02-08T18:53:57Z',
        Dirty: false,
        Note: 'pulld in the new world',
        Services: [
          'pulld.service',
        ],
      },
      {
        Name: 'pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-08T17:57:10Z:3827a3c53a66bee18070a17037282951b859b054.deb',
        Hash: '3827a3c53a66bee18070a17037282951b859b054',
        UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
        Built: '2018-02-08T17:57:10Z',
        Dirty: true,
        Note: 'test new world 3',
        Services: [
          'pulld.service',
        ],
      },
      {
        Name: 'pulld/pulld:jcgregorio@jcgregorio.cnc.corp.google.com:2018-02-08T17:49:13Z:3827a3c53a66bee18070a17037282951b859b054.deb',
        Hash: '3827a3c53a66bee18070a17037282951b859b054',
        UserID: 'jcgregorio@jcgregorio.cnc.corp.google.com',
        Built: '2018-02-08T17:49:13Z',
        Dirty: true,
        Note: 'test new world 3',
        Services: [
          'pulld.service',
        ],
      },
    ],
  },
  status: {
    'afdo-chromium-autoroll:git-cookie-authdaemon.service': {
      status: {
        Name: 'git-cookie-authdaemon.service',
        Description: 'Keeps the git cookie fresh.',
        LoadState: 'loaded',
        ActiveState: 'active',
        SubState: 'running',
        Followed: '',
        Path: '/org/freedesktop/systemd1/unit/git_2dcookie_2dauthdaemon_2eservice',
        JobId: 0,
        JobType: '',
        JobPath: '/',
      },
      props: {
        ExecMainStartTimestamp: 1518110861315026,
      },
    },
    'afdo-chromium-autoroll:pulld.service': {
      status: {
        Name: 'pulld.service',
        Description: 'Skia systemd monitoring UI and pull service.',
        LoadState: 'loaded',
        ActiveState: 'active',
        SubState: 'running',
        Followed: '',
        Path: '/org/freedesktop/systemd1/unit/pulld_2eservice',
        JobId: 0,
        JobType: '',
        JobPath: '/',
      },
      props: {
        ExecMainStartTimestamp: 1518116344218558,
      },
    },
    'android-compile:git-cookie-authdaemon.service': {
      status: {
        Name: 'git-cookie-authdaemon.service',
        Description: 'Keeps the git cookie fresh.',
        LoadState: 'loaded',
        ActiveState: 'active',
        SubState: 'running',
        Followed: '',
        Path: '/org/freedesktop/systemd1/unit/git_2dcookie_2dauthdaemon_2eservice',
        JobId: 0,
        JobType: '',
        JobPath: '/',
      },
      props: {
        ExecMainStartTimestamp: 1518454074747541,
      },
    },
    'android-compile:pulld.service': {
      status: {
        Name: 'pulld.service',
        Description: 'Skia systemd monitoring UI and pull service.',
        LoadState: 'loaded',
        ActiveState: 'active',
        SubState: 'running',
        Followed: '',
        Path: '/org/freedesktop/systemd1/unit/pulld_2eservice',
        JobId: 0,
        JobType: '',
        JobPath: '/',
      },
      props: {
        ExecMainStartTimestamp: 1518116379196476,
      },
    },
  },
};
ele._setState(state);

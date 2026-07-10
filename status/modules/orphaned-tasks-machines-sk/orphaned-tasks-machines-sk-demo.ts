import './index';
import fetchMock from 'fetch-mock';
import { Status, EMail } from '../../../infra-sk/modules/json';
import { Report } from './orphaned-tasks-machines-sk';

const loginStatus: Status = {
  email: 'user@google.com' as EMail,
  roles: ['admin'],
};

fetchMock.get('/loginstatus/', loginStatus);

const mockReport: Report = {
  no_matching_machines: [
    {
      dimensions: ['os:Ubuntu-20.04', 'cpu:x86-64', 'pool:Skia'],
      tasks: [
        'Test-Ubuntu20-Clang-GCE-GPU-Tegra3-x86_64-Debug-All',
        'Test-Ubuntu20-Clang-GCE-CPU-AVX2-x86_64-Release-All',
      ],
      machines: [],
      last_task_id: 'abc1234567890',
    },
    {
      dimensions: ['os:iOS-14', 'device:iPhone11', 'pool:Skia'],
      tasks: ['Test-iOS14-Clang-iPhone11-GPU-AppleA13-arm64-Release-All'],
      machines: [],
      last_task_id: '',
    },
  ],
  no_matching_tasks: [
    {
      dimensions: ['os:Android', 'device:sailfish', 'pool:Skia'],
      tasks: [],
      machines: ['skia-i-android-sailfish-001', 'skia-i-android-sailfish-002'],
      last_task_id: '',
    },
  ],
  timestamp: new Date('2026-07-08 15:04:05 UTC'),
};

fetchMock.get('/json/orphaned-tasks-machines', mockReport);

const el = document.createElement('orphaned-tasks-machines-sk');
document.querySelector('#container')?.appendChild(el);

import '../../../elements-sk/modules/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/alogin-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ARBStatusSk } from './arb-status-sk';
import { SetupMocks, GetFakeStatus } from '../rpc-mock';
import './index';

import { Status } from '../../../infra-sk/modules/json';

const status: Status = {
  email: 'user@google.com',
  roles: ['admin'],
};

fetchMock.get('/_/login/status', status);

SetupMocks();

document.querySelector('.component-goes-here')!.innerHTML =
  `<arb-status-sk></arb-status-sk>`;

// Get the name of the fake roller from the demo data.
const ele = <ARBStatusSk>document.getElementsByTagName('arb-status-sk')[0];
ele.roller = GetFakeStatus().config?.rollerId || '';

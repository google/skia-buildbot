import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ARBStatusSk } from './arb-status-sk';
import { SetupMocks, GetFakeStatus } from '../rpc-mock';

import './index.ts';

fetchMock.get('/loginstatus/', {
  Email: 'user@google.com',
  LoginURL: 'https://accounts.google.com/',
  IsAGoogler: true,
});
SetupMocks();

// Get the name of the fake roller from the demo data.
const ele = <ARBStatusSk>document.getElementsByTagName('arb-status-sk')[0];
ele.roller = GetFakeStatus().config?.rollerId || '';

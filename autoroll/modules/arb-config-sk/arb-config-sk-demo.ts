import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ARBConfigSk } from './arb-config-sk';
import { SetupMocks, GetFakeStatus } from '../rpc-mock';

fetchMock.get('/loginstatus/', {
  Email: 'user@google.com',
  LoginURL: 'https://accounts.google.com/',
  IsAGoogler: true,
});
SetupMocks();

import './index.ts';

// Get the name of the fake roller from the demo data.
const ele = <ARBConfigSk>document.getElementsByTagName('arb-config-sk')[0];
ele.roller = GetFakeStatus().config?.rollerId || '';

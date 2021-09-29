import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ARBConfigSk } from './arb-config-sk';
import { SetupMocks, GetFakeStatus } from '../rpc-mock';
import { GetFakeConfig } from '../rpc-mock/fake-config';

import './index.ts';

fetchMock.get('/loginstatus/', {
  Email: 'user@google.com',
  LoginURL: 'https://accounts.google.com/',
  IsAGoogler: true,
});
fetchMock.get('/r/skia-skiabot-test/config', GetFakeConfig());
SetupMocks();

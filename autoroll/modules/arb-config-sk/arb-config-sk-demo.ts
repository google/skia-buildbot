import '../../../elements-sk/modules/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/alogin-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { SetupMocks } from '../rpc-mock';
import { GetFakeConfig } from '../rpc-mock/fake-config';
import { Status } from '../../../infra-sk/modules/json';
import './index';

const status: Status = {
  email: 'user@google.com',
  roles: ['admin'],
};

fetchMock.get('/_/login/status', status);
fetchMock.get('/r/skia-skiabot-test/config', GetFakeConfig());
SetupMocks();

document.querySelector('.component-goes-here')!.innerHTML =
  '<arb-config-sk></arb-config-sk>';

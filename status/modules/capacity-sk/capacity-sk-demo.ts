/* eslint-disable no-use-before-define */
import './index';
import fetchMock from 'fetch-mock';
import { SetupMocks } from '../rpc-mock';
import { resp } from './test-data';

const loginStatus: Status = {
  email: 'user@google.com' as EMail,
  roles: ['admin'],
};

fetchMock.get('/loginstatus/', loginStatus);

SetupMocks().expectGetBotUsage(resp);
const el = document.createElement('capacity-sk');
document.querySelector('#container')?.appendChild(el);

import { Status, EMail } from '../../../infra-sk/modules/json';

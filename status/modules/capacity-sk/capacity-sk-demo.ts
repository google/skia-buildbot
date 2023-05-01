/* eslint-disable no-use-before-define */
/* eslint-disable import/first */
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

import './index';
import { Status, EMail } from '../../../infra-sk/modules/json';

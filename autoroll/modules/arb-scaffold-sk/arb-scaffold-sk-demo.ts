import fetchMock from 'fetch-mock';

import { Status } from '../../../infra-sk/modules/json';

const status: Status = {
  email: 'user@google.com',
  roles: ['admin'],
};

fetchMock.get('/_/login/status', status);

// eslint-disable-next-line import/first
import './index';

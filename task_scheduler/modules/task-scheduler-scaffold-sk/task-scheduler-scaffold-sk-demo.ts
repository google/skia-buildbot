import fetchMock from 'fetch-mock';
import { defaultStatusURL } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status } from '../../../infra-sk/modules/json';

const status: Status = {
  email: 'somebody@example.com',
  roles: ['viewer', 'admin'],
};

fetchMock.get(defaultStatusURL, status);

// eslint-disable-next-line import/first
import './index';

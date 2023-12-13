import fetchMock from 'fetch-mock';
import { Status } from '../../../infra-sk/modules/json';
import './index';

const status: Status = {
  email: 'user@google.com',
  roles: ['admin'],
};

fetchMock.get('/_/login/status', status);

document.querySelector('.component-goes-here')!.innerHTML = `
  <arb-scaffold-sk title="arb-scaffold-sk demo">
    <main>Content goes here.</main>
  </arb-scaffold-sk>
`;

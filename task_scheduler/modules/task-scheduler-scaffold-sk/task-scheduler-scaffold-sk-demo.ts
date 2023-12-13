import './index';
import fetchMock from 'fetch-mock';
import { defaultStatusURL } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status } from '../../../infra-sk/modules/json';

const status: Status = {
  email: 'somebody@example.com',
  roles: ['viewer', 'admin'],
};

fetchMock.get(defaultStatusURL, status);

document.querySelector('.component-goes-here')!.innerHTML = `
<task-scheduler-scaffold-sk title="task-scheduler-scaffold-sk demo">
  <main>Content goes here.</main>
</task-scheduler-scaffold-sk>
`;

import './index';
// eslint-disable-next-line import/no-extraneous-dependencies
import fetchMock from 'fetch-mock';
import { Status } from '../json';

const response: Status = {
  email: 'user@google.com',
  roles: ['viewer'],
};
fetchMock.get('/_/login/status', response);
fetchMock.get('/loginstatus/', response);

fetchMock.get('/this/should/return/a/404', 404);

document.querySelector('.fetching-example-goes-here')!.innerHTML =
  `<alogin-sk></alogin-sk>`;
document.querySelector('.testing-example-goes-here')!.innerHTML =
  `<alogin-sk testing_offline></alogin-sk>`;
document.querySelector('.failure-example-goes-here')!.innerHTML =
  `<alogin-sk url="/this/should/return/a/404"></alogin-sk>`;

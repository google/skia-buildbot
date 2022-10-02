// eslint-disable-next-line import/no-extraneous-dependencies
import fetchMock from 'fetch-mock';
import { Status } from '../json';

const response: Status = {
  email: 'user@google.com',
  login: 'https://skia.org/login/',
  logout: 'https://skia.org/logout/',
  roles: ['viewer'],
};
fetchMock.get('/_/login/status', response);

fetchMock.get('/this/should/return/a/404', 404);

// eslint-disable-next-line import/first
import './index';

// eslint-disable-next-line import/no-extraneous-dependencies
import fetchMock from 'fetch-mock';
import { alogin } from '../json';

const response: alogin.Status = {
  email: 'user@google.com',
  login: 'https://skia.org/login/',
  logout: 'https://skia.org/logout/',
};
fetchMock.get('/_/login/status', response);

// eslint-disable-next-line import/first
import './index';

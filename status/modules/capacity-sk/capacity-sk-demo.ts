import fetchMock from 'fetch-mock';
import { SetupMocks } from '../rpc-mock';
import './index';
import { resp } from './test-data';

fetchMock.get('/loginstatus/', {
  Email: 'user@google.com',
});

SetupMocks().expectGetBotUsage(resp);
const el = document.createElement('capacity-sk');
document.querySelector('#container')?.appendChild(el);

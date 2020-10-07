import './index';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';
import { $$ } from 'common-sk/modules/dom';

SetupMocks().expectGetIncrementalCommits(mockIncrementalResponse);

const data = document.createElement('status-sk');
($$('#container') as HTMLElement).appendChild(data);

document.querySelector('status-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});

import fetchMock from 'fetch-mock';
import { RevisionInfo } from '../json';
import './index';

document
  .querySelector('revision-info-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  '
    );
  });
const revId = '12345';
const response: RevisionInfo[] = [
  {
    benchmark: 'b1',
    bot: 'bot1',
    bug_id: '111',
    end_revision: 456,
    start_revision: 123,
    explore_url: 'https://url',
    is_improvement: false,
    master: 'm1',
    test: 't1',
  },
];

fetchMock.get(`/_/revision/?rev=${revId}`, response);

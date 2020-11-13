
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { startRequest } from './progress';
import { progress } from '../json';
import 'elements-sk/spinner-sk';

describe('startRequest', () => {
  const spinner = document.createElement('spinner-sk') as SpinnerSk;
  document.body.appendChild(spinner);
  it('handles the first request returing as Finished', async () => {
    const url = '/_/list/start';
    const finishedBody: progress.SerializedProgress = {
      status: 'Finished',
      messages: [{ key: 'Step', value: '2/2' }],
      results: { somedata: 1 },
      url: '',
    };
    fetchMock.post(url, finishedBody);
    const res = await startRequest(url, {}, 1, spinner);
    assert.deepEqual(res, finishedBody);
  });
});

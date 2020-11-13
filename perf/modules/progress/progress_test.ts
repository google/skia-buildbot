
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { startRequest } from './progress';
import { progress } from '../json';
import 'elements-sk/spinner-sk';

fetchMock.config.overwriteRoutes = true;

const startURL = '/_/list/start';
const pollingURL = '/_/list/status';

const finishedBody: progress.SerializedProgress = {
  status: 'Finished',
  messages: [{ key: 'Step', value: '2/2' }],
  results: { somedata: 1 },
  url: '',
};

const intermediateStepBody: progress.SerializedProgress = {
  status: 'Running',
  messages: [{ key: 'Step', value: '1/2' }],
  url: pollingURL,
};


describe('startRequest', () => {
  // Create a common spinner-sk to be used by all the tests.
  const spinner = document.createElement('spinner-sk') as SpinnerSk;
  document.body.appendChild(spinner);

  it('handles the first request returing as Finished', async () => {
    fetchMock.post(startURL, finishedBody);
    const res = await startRequest(startURL, {}, 1, spinner);
    assert.deepEqual(res, finishedBody);
  });

  it('starts polling the URL returned in the SerializedProgress', async () => {
    fetchMock.post(startURL, intermediateStepBody);
    fetchMock.get(pollingURL, finishedBody);
    const res = await startRequest(startURL, {}, 1, spinner);
    assert.deepEqual(res, finishedBody);
  });
});

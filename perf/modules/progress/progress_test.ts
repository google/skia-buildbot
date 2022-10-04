import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { messageByName, messagesToErrorString, startRequest } from './progress';
import { progress } from '../json/all';
import 'elements-sk/spinner-sk';

fetchMock.config.overwriteRoutes = true;

const startURL = '/_/list/start';
const pollingURL = '/_/list/status';
const finalURL = '';

const finishedBody: progress.SerializedProgress = {
  status: 'Finished',
  messages: [{ key: 'Step', value: '2/2' }],
  results: { somedata: 1 },
  url: finalURL,
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
    const res = await startRequest(startURL, {}, 1, spinner, null);
    assert.deepEqual(res, finishedBody);
  });

  it('starts polling the URL returned in the SerializedProgress', async () => {
    fetchMock.post(startURL, intermediateStepBody);
    fetchMock.get(pollingURL, finishedBody);
    const res = await startRequest(startURL, {}, 1, spinner, null);
    assert.deepEqual(res, finishedBody);
  });

  it('rejects the returned Promise on a fetch error', async () => {
    fetchMock.post(startURL, 500);
    try {
      await startRequest(startURL, {}, 1, spinner, null);
      assert.fail('Should never get here.');
    } catch (err) {
      assert.match((err as Error).message, /Bad network response/);
    }
  });

  it('triggers the callback every time it polls', async () => {
    fetchMock.post(startURL, intermediateStepBody);
    fetchMock.get(pollingURL, finishedBody);

    // Create a callback, and inside it check that the right arg is being sent
    // each time.
    let index = 0;
    const callbackBodies = [intermediateStepBody, finishedBody];
    const cb = (sp: progress.SerializedProgress) => {
      assert.deepEqual(sp, callbackBodies[index]);
      index++;
    };

    const res = await startRequest(startURL, {}, 1, spinner, cb);
    assert.deepEqual(res, finishedBody);
    assert.equal(index, 2);
  });
});

describe('messageToErrorString', () => {
  it('converts an empty messages into an error string', () => {
    assert.equal('(no error message available)', messagesToErrorString([]));
  });

  it('uses the Error value if available', () => {
    assert.equal('Error: This is my error', messagesToErrorString([{ key: 'Error', value: 'This is my error' }]));
  });

  it('falls back to using all the key/value pairs if Error is not present', () => {
    assert.equal('Step: 1/2 Status: Querying database', messagesToErrorString([{ key: 'Step', value: '1/2' }, { key: 'Status', value: 'Querying database' }]));
  });
});

describe('messageByName', () => {
  it('converts an empty messages into the default string', () => {
    const defaultValue = 'some-default-value';
    assert.equal(defaultValue, messageByName([], 'SomeKeyName', defaultValue));
  });

  it('retrieves the message if it exists', () => {
    assert.equal('This is the value', messageByName([{ key: 'SomeKey', value: 'This is the value' }], 'SomeKey'));
  });

  it('retrieves the default string if the key does not exist', () => {
    assert.equal('', messageByName([{ key: 'SomeKey', value: 'This is the value' }], 'NotAKeyInTheMessages'));
  });
});

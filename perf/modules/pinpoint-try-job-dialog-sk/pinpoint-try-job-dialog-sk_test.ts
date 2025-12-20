import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { PinpointTryJobDialogSk } from './pinpoint-try-job-dialog-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { resetLoggedInPromise } from '../../../infra-sk/modules/alogin-sk/alogin-sk';

describe('pinpoint-try-job-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<PinpointTryJobDialogSk>('pinpoint-try-job-dialog-sk');

  let element: PinpointTryJobDialogSk;
  beforeEach(async () => {
    resetLoggedInPromise();
    fetchMock.get('/_/login/status', { email: 'user@google.com', roles: [] });
    element = newInstance();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    fetchMock.restore();
  });

  it('renders with default values', () => {
    assert.isNotNull(element.querySelector('#pinpoint-try-job-dialog'));
    const baseCommitInput = element.querySelector('#base-commit') as HTMLInputElement;
    assert.equal(baseCommitInput.value, '');
  });

  it('sets parameters correctly', () => {
    element.setTryJobInputParams({
      testPath: 'master/bot/benchmark/test/story',
      baseCommit: 'abc',
      endCommit: 'def',
      story: 'story',
    });

    assert.equal((element.querySelector('#base-commit') as HTMLInputElement).value, 'abc');
    assert.equal((element.querySelector('#exp-commit') as HTMLInputElement).value, 'def');
  });

  it('posts a try job', async () => {
    element.setTryJobInputParams({
      testPath: 'master/bot/benchmark/test/story',
      baseCommit: 'abc',
      endCommit: 'def',
      story: 'story',
    });

    fetchMock.post('/_/try/', { jobUrl: 'http://pinpoint/123' });

    // Simulate form submission
    const form = element.querySelector('#pinpoint-try-job-form') as HTMLFormElement;
    form.dispatchEvent(new Event('submit'));

    // Wait for fetch to complete and for lit-html to render.
    await fetchMock.flush(true);

    assert.isTrue(fetchMock.called('/_/try/'));
    const lastCall = fetchMock.lastCall('/_/try/');
    const body = JSON.parse(lastCall![1]!.body as string);
    assert.equal(body.base_git_hash, 'abc');
    assert.equal(body.end_git_hash, 'def');
    assert.equal(body.benchmark, 'benchmark');
    assert.equal(body.story, 'story');
    assert.equal(body.user, 'user@google.com');

    // Check if job link is rendered
    assert.isNotNull(element.querySelector('a[href="http://pinpoint/123"]'));
  });
});

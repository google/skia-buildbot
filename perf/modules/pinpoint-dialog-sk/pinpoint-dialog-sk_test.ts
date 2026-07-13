import './pinpoint-dialog-sk';
import { assert } from 'chai';
import { PinpointDialogSk, PinpointPreloadParams } from './pinpoint-dialog-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

describe('pinpoint-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<PinpointDialogSk>('pinpoint-dialog-sk');
  let element: PinpointDialogSk;

  beforeEach(async () => {
    fetchMock.get('/_/login/status', { email: 'test@example.com' });
    element = newInstance();
    await element.updateComplete;
  });

  afterEach(() => {
    delete (window as any).perf;
    fetchMock.restore();
    sinon.restore();
  });

  describe('preload params', () => {
    it('binds inputs correctly in bisect mode', () => {
      const params: PinpointPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/subtest',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '999',
        story: 'subtest',
        anomalyId: 'a1',
      };
      element.open('bisect', params);

      assert.equal(element.mode, 'bisect');
      assert.equal(element.testPath, params.testPath);
      assert.equal(element.bugId, params.bugId);
      assert.equal(element.startCommit, params.startCommit);
      assert.equal(element.endCommit, params.endCommit);
      assert.equal(element.story, params.story);
    });
  });

  describe('tabs toggle', () => {
    it('switches modes on clicking tabs', async () => {
      element.open('bisect', {});
      await element.updateComplete;

      assert.equal(element.mode, 'bisect');

      // Toggle tab
      element.mode = 'try';
      await element.updateComplete;
      assert.equal(element.mode, 'try');
    });
  });

  describe('validations and submit bisection', () => {
    it('submits correct payload to API', async () => {
      const params: PinpointPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/subtest',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '999',
        story: 'subtest',
        anomalyId: 'a1',
      };
      element.open('bisect', params);
      element.user = 'test@example.com';
      await element.updateComplete;

      fetchMock.post('/_/bisect/create', { jobId: 'job123', jobUrl: '/job/123' });

      element.submitBisect();
      await fetchMock.flush(true);

      const lastOptions = fetchMock.lastOptions('/_/bisect/create');
      assert.isDefined(lastOptions);
      const body = JSON.parse(lastOptions!.body as unknown as string);
      assert.equal(body.bug_id, '999');
      assert.equal(body.start_git_hash, '12345');
      assert.equal(body.story, 'subtest');
    });
  });

  describe('new pinpoint checkbox visibility', () => {
    it('shows checkbox when show_new_pinpoint_backend_checkbox is true', async () => {
      (window as any).perf = { show_new_pinpoint_backend_checkbox: true };
      element.open('bisect', {});
      element.requestUpdate();
      await element.updateComplete;
      const checkbox = element.shadowRoot!.querySelector('#use-new-pinpoint');
      assert.isNotNull(checkbox);
    });

    it('hides checkbox when show_new_pinpoint_backend_checkbox is false', async () => {
      (window as any).perf = { show_new_pinpoint_backend_checkbox: false };
      element.open('bisect', {});
      element.requestUpdate();
      await element.updateComplete;
      const checkbox = element.shadowRoot!.querySelector('#use-new-pinpoint');
      assert.isNull(checkbox);
    });
  });
});

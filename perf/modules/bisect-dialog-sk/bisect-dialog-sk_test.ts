import './bisect-dialog-sk';
import { assert } from 'chai';
import { BisectDialogSk, BisectPreloadParams } from './bisect-dialog-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { CreateBisectRequest } from '../json';

describe('bisect-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<BisectDialogSk>('bisect-dialog-sk');

  let element: BisectDialogSk;
  let dialog: HTMLDialogElement;

  beforeEach(() => {
    // Mock LoggedIn() to return a test user.
    fetchMock.get('/_/login/status', { email: 'test@example.com' });
    element = newInstance();
    dialog = element.querySelector('#bisect-dialog')!;
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  describe('setBisectInputParams', () => {
    it('sets the properties correctly', () => {
      const params: BisectPreloadParams = {
        testPath: 'master/bot/test_suite/test/trace',
        startCommit: 'c1',
        endCommit: 'c2',
        bugId: '123',
        story: 'story',
        anomalyId: 'a1',
      };
      element.setBisectInputParams(params);

      assert.equal(element.testPath, params.testPath);
      assert.equal(element.startCommit, params.startCommit);
      assert.equal(element.endCommit, params.endCommit);
      assert.equal(element.bugId, params.bugId);
      assert.equal(element.story, params.story);
      assert.equal(element.anomalyId, params.anomalyId);
    });
  });

  describe('open and close', () => {
    it('opens and closes the dialog', () => {
      const showModalStub = sinon.stub(dialog, 'showModal');
      const closeStub = sinon.stub(dialog, 'close');

      element.open();
      assert.isTrue(showModalStub.calledOnce);

      (element as any).closeBisectDialog();
      assert.isTrue(closeStub.calledOnce);
    });
  });

  describe('postBisect', () => {
    it('does nothing if testPath is empty', async () => {
      element.testPath = '';
      await (element as any).postBisect();
      assert.isFalse(fetchMock.called());
    });

    it('sends a bisect request with statistic', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/trace:avg',
        startCommit: 'c1',
        endCommit: 'c2',
        bugId: '123',
        anomalyId: 'a1',
      };
      element.setBisectInputParams(params);
      (element as any).user = 'test@example.com';

      fetchMock.post('/_/bisect/create', { jobId: 'job1', jobUrl: '/job/1' });

      element.postBisect();
      await fetchMock.flush(true);

      const bisectRequest = fetchMock.lastOptions('/_/bisect/create');
      const bisectBody = JSON.parse(
        bisectRequest!.body as unknown as string
      ) as CreateBisectRequest;

      assert.equal(bisectBody.bug_id, '123');
      assert.equal(bisectBody.start_git_hash, 'c1');
      assert.equal(bisectBody.end_git_hash, 'c2');
      assert.equal(bisectBody.chart, 'test_suite');
      assert.equal(bisectBody.statistic, '');
      assert.equal(bisectBody.story, 'trace:avg');
      assert.equal(bisectBody.alert_ids, '[a1]');
      assert.equal(bisectBody.project, 'chromium');
      assert.equal(bisectBody.comparison_mode, 'performance');
      assert.equal(bisectBody.configuration, 'MacM1');
      assert.equal(bisectBody.benchmark, 'Blazor');
    });

    it('shows an error message on failure', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/test/trace',
        startCommit: 'c1',
        endCommit: 'c2',
        bugId: '123',
        story: 'story',
        anomalyId: 'a1',
      };
      element.setBisectInputParams(params);
      element.user = 'test@example.com';

      fetchMock.post('/_/bisect/create', 500);
      const event = eventPromise('error-sk');
      element.postBisect();
      await fetchMock.flush(true);

      const errEvent = await event;
      const errMessage = (errEvent as CustomEvent).detail.message as string;

      assert.isDefined(errEvent);
      assert.isNotNull(errMessage);
    });
  });

  describe('reset', () => {
    it('clears the properties', () => {
      const params: BisectPreloadParams = {
        testPath: 'master/bot/test_suite/test/trace',
        startCommit: 'c1',
        endCommit: 'c2',
        bugId: '123',
        story: 'story',
        anomalyId: 'a1',
      };
      element.setBisectInputParams(params);

      element.reset();

      assert.equal(element.testPath, '');
      assert.equal(element.startCommit, '');
      assert.equal(element.endCommit, '');
      assert.equal(element.bugId, '123'); // bugId is not cleared in reset()
      assert.equal(element.story, '');
      assert.equal(element.anomalyId, '');
    });
  });
});

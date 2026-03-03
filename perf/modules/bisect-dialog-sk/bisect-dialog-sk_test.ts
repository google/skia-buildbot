import './bisect-dialog-sk';
import { assert } from 'chai';
import { BisectDialogSk, BisectPreloadParams } from './bisect-dialog-sk';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { BisectJobCreateRequest } from '../json';

describe('bisect-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<BisectDialogSk>('bisect-dialog-sk');

  let element: BisectDialogSk;
  let dialog: HTMLDialogElement;

  beforeEach(async () => {
    // Mock LoggedIn() to return a test user.
    fetchMock.get('/_/login/status', { email: 'test@example.com' });
    element = newInstance();
    await element.updateComplete;
    dialog = element.querySelector('#bisect-dialog')!;
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  describe('setBisectInputParams', () => {
    it('sets the properties correctly', () => {
      const params: BisectPreloadParams = {
        testPath: 'master/bot/test_suite/test/subtest',
        startCommit: '12345',
        endCommit: '12346',
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
    it('does not send a bisect request if testPath is empty', async () => {
      element.startCommit = 'c1';
      element.endCommit = 'c2';
      element.bugId = '123';
      element.testPath = ''; // Explicitly empty
      element.postBisect();
      assert.isFalse(fetchMock.called());
    });

    it('sends a bisect request with statistic', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/subtest:avg',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '123',
        anomalyId: 'a1',
      };
      await element.setBisectInputParams(params);
      await element.updateComplete;
      element.user = 'test@example.com';
      element.open();
      await element.updateComplete;

      fetchMock.post('/_/bisect/create', { jobId: 'job1', jobUrl: '/job/1' });

      await element.postBisect();
      await fetchMock.flush(true);
      const bisectRequest = fetchMock.lastOptions('/_/bisect/create');
      const bisectBody = JSON.parse(
        bisectRequest!.body as unknown as string
      ) as BisectJobCreateRequest;

      const expected: BisectJobCreateRequest = {
        bug_id: '123',
        start_git_hash: '12345',
        end_git_hash: '12346',
        chart: 'test_suite',
        statistic: '',
        story: 'subtest_avg',
        alert_ids: '[a1]',
        project: 'chromium',
        comparison_mode: 'performance',
        configuration: 'MacM1',
        benchmark: 'Blazor',
        test_path: 'ChromiumPerf/MacM1/Blazor/test_suite/subtest:avg',
        comparison_magnitude: '',
        pin: '',
        user: 'test@example.com',
      };
      assert.deepEqual(bisectBody, expected);
    });

    it('replaces colons in story', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/subtest:with:colons',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '123',
        anomalyId: 'a1',
      };
      await element.setBisectInputParams(params);
      await element.updateComplete;
      element.user = 'test@example.com';
      element.open();
      await element.updateComplete;

      fetchMock.post('/_/bisect/create', { jobId: 'job1', jobUrl: '/job/1' });

      await element.postBisect();
      await fetchMock.flush(true);
      const bisectRequest = fetchMock.lastOptions('/_/bisect/create');
      const bisectBody = JSON.parse(
        bisectRequest!.body as unknown as string
      ) as BisectJobCreateRequest;

      assert.equal(bisectBody.story, 'subtest_with_colons');
    });

    it('does not replace colons in chart', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test:suite:with:colons/subtest',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '123',
        anomalyId: 'a1',
      };
      await element.setBisectInputParams(params);
      await element.updateComplete;
      element.user = 'test@example.com';
      element.open();
      await element.updateComplete;

      fetchMock.post('/_/bisect/create', { jobId: 'job1', jobUrl: '/job/1' });

      await element.postBisect();
      await fetchMock.flush(true);
      const bisectRequest = fetchMock.lastOptions('/_/bisect/create');
      const bisectBody = JSON.parse(
        bisectRequest!.body as unknown as string
      ) as BisectJobCreateRequest;

      assert.equal(bisectBody.chart, 'test:suite:with:colons');
    });

    it('shows an error message on failure', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/test/subtest',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '123',
        story: 'story',
        anomalyId: 'a1',
      };
      element.setBisectInputParams(params);
      await element.updateComplete;
      element.user = 'test@example.com';
      element.open();
      await element.updateComplete;

      fetchMock.post('/_/bisect/create', 500);
      const event = eventPromise('error-sk');
      element.postBisect();
      await fetchMock.flush(true);

      const errEvent = await event;
      const errMessage = (errEvent as CustomEvent).detail.message as string;

      assert.isDefined(errEvent);
      assert.isNotNull(errMessage);
    });

    it('displays a link to the pinpoint job after creation', async () => {
      const params: BisectPreloadParams = {
        testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/test/subtest',
        startCommit: '12345',
        endCommit: '12346',
        bugId: '123',
        story: 'story',
        anomalyId: 'a1',
      };
      await element.setBisectInputParams(params);
      element.user = 'test@example.com';
      element.open();
      await element.updateComplete;

      const jobUrl = 'https://pinpoint-dot-chromeperf.appspot.com/job/12345';
      fetchMock.post('/_/bisect/create', { jobId: '12345', jobUrl: jobUrl });

      await element.postBisect();
      await fetchMock.flush(true);

      const link = element.querySelector('#pinpoint-job-url') as HTMLAnchorElement;
      assert.isNotNull(link);
      assert.equal(link!.href, jobUrl);
      assert.include(link!.textContent, 'Bisect job created');
    });
  });

  describe('request parameter validation', async () => {
    const validParams: BisectPreloadParams = {
      testPath: 'ChromiumPerf/MacM1/Blazor/test_suite/test/subtest',
      startCommit: '12345',
      endCommit: '12346',
      bugId: '123',
      story: 'story',
      anomalyId: 'a1',
    };

    const testCases: {
      paramsOverride: Partial<BisectPreloadParams>;
      expectedError: string;
      testName: string;
    }[] = [
      {
        paramsOverride: { startCommit: '' },
        testName: 'start commit',
        expectedError: 'Start commit is required.',
      },
      {
        paramsOverride: { endCommit: '' },
        testName: 'end commit',
        expectedError: 'End commit is required.',
      },
      {
        // 'src//bench/test/story'. [1] is ''.
        paramsOverride: { testPath: 'src//bench/test/story' },
        testName: 'configuration',
        expectedError: 'Configuration is missing in the request.',
      },
      {
        // 'src/cfg//test/story'. [2] is ''.
        paramsOverride: { testPath: 'src/cfg//test/story' },
        testName: 'benchmark',
        expectedError: 'Benchmark is missing in the request.',
      },
      {
        // 'src/cfg/bench/test/'. pop() is ''.
        paramsOverride: { testPath: 'src/cfg/bench/test/' },
        testName: 'story',
        expectedError: 'Story is missing in the request.',
      },
      {
        // 'src/cfg/bench//story'. at(3) is ''.
        paramsOverride: { testPath: 'src/cfg/bench//story' },
        testName: 'chart',
        expectedError: 'Chart is missing in the request.',
      },
      {
        paramsOverride: { testPath: '' },
        testName: 'test path',
        expectedError: 'Test path is missing in the request.',
      },
    ];

    it('shows an error message if a required parameter is missing', async function () {
      this.timeout(10000);
      for (const { paramsOverride, expectedError, testName } of testCases) {
        const params: BisectPreloadParams = {
          ...validParams,
          ...paramsOverride,
        };

        await element.setBisectInputParams(params);
        await element.updateComplete;
        element.open();
        await element.updateComplete;
        const event = eventPromise('error-sk');
        await element.postBisect();

        const errEvent = await event;
        const errMessage = (errEvent as CustomEvent).detail.message as string;

        assert.isFalse(fetchMock.called(), `Fetch called for ${testName}`);
        assert.equal(errMessage, expectedError, `Wrong error for ${testName}`);
        dialog.close();
      }
    });
  });

  describe('reset', () => {
    it('clears the properties', () => {
      const params: BisectPreloadParams = {
        testPath: 'master/bot/test_suite/test/subtest',
        startCommit: '12345',
        endCommit: '12346',
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
      assert.equal((element as any).patch, '');
    });
  });
});

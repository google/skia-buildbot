import { expect, assert } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  EventPromiseFactory,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { BisectDialogSkPO } from './bisect-dialog-sk_po';
import { anomalies } from './test_data';

describe('bisect-dialog-sk', () => {
  let testBed: TestBed;
  let bisectDialogSkPO: BisectDialogSkPO;
  const mockResponses = {
    '/_/login/status': {
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ email: 'test@google.com', Roles: ['editor'] }),
    },
    '/_/bisect/create': {
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ jobId: '123', jobUrl: 'http://example.com' }),
    },
  };

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setRequestInterception(true);

    testBed.page.on('request', (request) => {
      const matchingPath = Object.keys(mockResponses).find((path) => request.url().endsWith(path));

      if (matchingPath) {
        request.respond(mockResponses[matchingPath as keyof typeof mockResponses]);
      } else {
        request.continue();
      }
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 1000 });
    bisectDialogSkPO = new BisectDialogSkPO((await testBed.page.$('bisect-dialog-sk'))!);
  });

  afterEach(async () => {
    testBed.page.removeAllListeners('request');
    await testBed.page.setRequestInterception(false);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('bisect-dialog-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'bisect-dialog-sk');
    });
  });

  describe('dialog interaction', () => {
    beforeEach(async () => {
      // Assuming there is a button on the demo page to open the dialog.
      await testBed.page.click('#show-dialog');
    });

    it('opens the dialog', async () => {
      assert.isTrue(await bisectDialogSkPO.isDialogOpen());
    });

    it('fills out the form', async () => {
      await bisectDialogSkPO.setTestPath(anomalies[0].test_path);
      await bisectDialogSkPO.setBugId('12345');
      await bisectDialogSkPO.setStartCommit(anomalies[0].start_revision.toString());
      await bisectDialogSkPO.setEndCommit(anomalies[0].end_revision.toString());
      await bisectDialogSkPO.setStory('async-fs');
      await bisectDialogSkPO.setPatch('patch');

      expect(await bisectDialogSkPO.getTestPath()).to.equal(
        'internal.client.v8/x64/v8/JetStream2/maglev-future/async-fs/Average'
      );
      expect(await bisectDialogSkPO.getBugId()).to.equal('12345');
      expect(await bisectDialogSkPO.getStartCommit()).to.equal('95942');
      expect(await bisectDialogSkPO.getEndCommit()).to.equal('95942');
      expect(await bisectDialogSkPO.getStory()).to.equal('async-fs');
      expect(await bisectDialogSkPO.getPatch()).to.equal('patch');
    });

    it('submits the form', async () => {
      assert.isTrue(await bisectDialogSkPO.isDialogOpen());
      await bisectDialogSkPO.setTestPath(anomalies[0].test_path);
      await bisectDialogSkPO.setBugId('12345');
      await bisectDialogSkPO.setStartCommit(anomalies[0].start_revision.toString());
      await bisectDialogSkPO.setEndCommit(anomalies[0].end_revision.toString());

      await bisectDialogSkPO.clickBisectBtn();
      assert.isTrue(await bisectDialogSkPO.isDialogOpen());
      await takeScreenshot(testBed.page, 'perf', 'bisect-dialog-sk-open');
    });
  });

  describe('when not logged in', () => {
    let eventPromise: EventPromiseFactory;

    beforeEach(async () => {
      // This test needs to set up its own mocks and navigate, so we
      // clear the listeners from the parent beforeEach.
      testBed.page.removeAllListeners('request');
      await testBed.page.setRequestInterception(true);

      eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['error-sk']);

      testBed.page.on('request', (request) => {
        // replace the login response with empty email
        if (request.url().endsWith('/_/login/status')) {
          request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ email: '' }),
          });
        } else {
          request.continue();
        }
      });
      await testBed.page.goto(testBed.baseUrl);
      await testBed.page.setViewport({ width: 600, height: 1000 });
      bisectDialogSkPO = new BisectDialogSkPO((await testBed.page.$('bisect-dialog-sk'))!);
    });

    it('shows an error toast when trying to submit', async () => {
      const errorEvent = eventPromise('error-sk');

      await testBed.page.click('#show-dialog');
      assert.isTrue(await bisectDialogSkPO.isDialogOpen());

      await bisectDialogSkPO.setTestPath(anomalies[0].test_path);
      await bisectDialogSkPO.setBugId('12345');
      await bisectDialogSkPO.setStartCommit(anomalies[0].start_revision.toString());
      await bisectDialogSkPO.setEndCommit(anomalies[0].end_revision.toString());

      await bisectDialogSkPO.clickBisectBtn();

      const errEvent = await errorEvent;
      const errMessage = (errEvent as any).message as string;

      assert.equal(errMessage, 'User is not logged in.');
    });
  });
});

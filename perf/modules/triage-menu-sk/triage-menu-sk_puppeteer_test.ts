import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { TriageMenuSkPO } from './triage-menu-sk_po';
import { STANDARD_LAPTOP_VIEWPORT } from '../common/puppeteer-test-util';
import { Page } from 'puppeteer';

describe('triage-menu-sk', () => {
  let testBed: TestBed;
  let triageMenuSkPO: TriageMenuSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setRequestInterception(true);

    testBed.page.on('request', (request) => {
      if (request.url().endsWith('/_/triage/edit_anomalies') && request.method() === 'POST') {
        request.respond({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({}),
        });
      } else if (request.url().endsWith('/_/triage/file_bug') && request.method() === 'POST') {
        request.respond({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ bug_id: 12345 }),
        });
      } else if (request.url().endsWith('/_/bug/associate') && request.method() === 'POST') {
        request.respond({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({}),
        });
      } else {
        request.continue();
      }
    });

    await testBed.page.goto(testBed.baseUrl);
    const triageMenuSk = (await testBed.page.$('triage-menu-sk'))!;
    triageMenuSkPO = new TriageMenuSkPO(triageMenuSk);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  afterEach(async () => {
    testBed.page.removeAllListeners('request');
    await testBed.page.setRequestInterception(false);
  });

  it('should render the demo page', async () => {
    await takeScreenshot(testBed.page, 'perf', 'triage-menu-sk');
    // Smoke test.
    expect(await testBed.page.$$('triage-menu-sk')).to.have.length(1);
  });

  it('should open new bug dialog when New Bug button is clicked', async () => {
    try {
      await triageMenuSkPO.newBugButton.click();
    } catch (e) {
      throw new Error(
        `Custom element "new-bug-dialog-sk" was not defined within the timeout. Error: ${
          e instanceof Error
        }`
      );
    }
  });

  it('should redirect to a new page when clicking New Bug button', async () => {
    const [newPage] = await Promise.all([
      new Promise<Page>((resolve) => testBed.page.once('popup', resolve)),
      triageMenuSkPO.newBugButton.click(),
    ]);

    await newPage.waitForNavigation();
    expect(newPage.url()).to.equal('https://issues.chromium.org/issues/12345');
  });

  it('should open existing bug dialog when Existing Bug button is clicked', async () => {
    try {
      await triageMenuSkPO.existingBugButton.click();
    } catch (e) {
      throw new Error(
        `Custom element "existing-bug-dialog-sk" was not defined within the timeout. Error: ${
          e instanceof Error
        }`
      );
    }
    expect(triageMenuSkPO.existingBugDialog.dialog).is.not.null;
  });

  it('should ignore anomalies when Ignore button is clicked', async () => {
    await triageMenuSkPO.ignoreButton.click();

    expect(triageMenuSkPO.ignoreToast).is.not.null;
    const closeIgnoreToastButton = triageMenuSkPO.ignoreToast.bySelector('#hide-ignore-triage')!;
    await closeIgnoreToastButton.click();
    expect(await triageMenuSkPO.ignoreToast.hasAttribute('hidden')).to.be.false;
  });
});

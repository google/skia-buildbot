import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { TriagePanelSkPO } from './triage-panel-sk_po';
import { STANDARD_LAPTOP_VIEWPORT } from '../common/puppeteer-test-util';
import { Page } from 'puppeteer';

describe('triage-panel-sk', () => {
  let testBed: TestBed;
  let triagePanelSkPO: TriagePanelSkPO;

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
      } else if (
        request.url().endsWith('/_/triage/associate_alerts') &&
        request.method() === 'POST'
      ) {
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
    await testBed.page.evaluate(() => localStorage.clear());
    await testBed.page.reload();

    const triagePanelSk = (await testBed.page.$('triage-panel-sk'))!;
    triagePanelSkPO = new TriagePanelSkPO(triagePanelSk);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  afterEach(async () => {
    testBed.page.removeAllListeners('request');
    await testBed.page.setRequestInterception(false);
  });

  it('should render the demo page', async () => {
    await takeScreenshot(testBed.page, 'perf', 'triage-panel-sk');
    expect(await testBed.page.$$('triage-panel-sk')).to.have.length(1);
  });

  it('should add a new bucket when inputting name and clicking Add Bucket', async () => {
    await triagePanelSkPO.newBucketInput.type('My Regression Bucket');
    await triagePanelSkPO.addBucketButton.click();
    const cards = await testBed.page.$$('.bucket-card');
    expect(cards).to.have.length(1);
    const title = await testBed.page.$eval(
      '.bucket-card h3',
      (el) => (el as HTMLElement).innerText
    );
    expect(title).to.include('My Regression Bucket');
  });

  it('should collapse and expand when collapse button is clicked', async () => {
    await triagePanelSkPO.collapseButton.click();
    expect(await testBed.page.$eval('triage-panel-sk', (el) => el.hasAttribute('collapsed'))).to.be
      .true;
    await triagePanelSkPO.collapseButton.click();
    expect(await testBed.page.$eval('triage-panel-sk', (el) => el.hasAttribute('collapsed'))).to.be
      .false;
  });

  it('should persist buckets to localStorage across page reloads', async () => {
    await triagePanelSkPO.newBucketInput.type('Persistent Bucket');
    await triagePanelSkPO.addBucketButton.click();
    expect(await testBed.page.$$('.bucket-card')).to.have.length(1);

    // Reload page to verify localStorage persistence.
    await testBed.page.reload();
    expect(await testBed.page.$$('.bucket-card')).to.have.length(1);
    const title = await testBed.page.$eval(
      '.bucket-card h3',
      (el) => (el as HTMLElement).innerText
    );
    expect(title).to.include('Persistent Bucket');
  });

  it('should copy all buckets to clipboard', async () => {
    await triagePanelSkPO.newBucketInput.type('Copy Bucket');
    await triagePanelSkPO.addBucketButton.click();

    const clipboardText = await testBed.page.evaluate(() => {
      const panel = document.querySelector<any>('triage-panel-sk');
      return panel.bucketsController.copyAll();
    });
    expect(clipboardText).to.include('[Copy Bucket]');
  });

  it('should perform Ignore triage action', async () => {
    await triagePanelSkPO.newBucketInput.type('Ignore Bucket');
    await triagePanelSkPO.addBucketButton.click();

    // Stage an anomaly via textarea.
    const textarea = (await testBed.page.$('.bucket-card .bucket-textarea'))!;
    await textarea.type('100:Master/Bot/Benchmark/Story/Metric1:100');

    const ignoreBtn = (await testBed.page.$('.bucket-card .ignore-btn'))!;
    await ignoreBtn.click();

    // Verify status badge updates.
    await testBed.page.waitForSelector('.bucket-card .status-badge');
    const status = await testBed.page.$eval(
      '.bucket-card .status-badge',
      (el) => (el as HTMLElement).innerText
    );
    expect(status).to.include('Ignored');
  });

  it('should perform New Bug triage action', async () => {
    await triagePanelSkPO.newBucketInput.type('New Bug Bucket');
    await triagePanelSkPO.addBucketButton.click();

    const textarea = (await testBed.page.$('.bucket-card .bucket-textarea'))!;
    await textarea.type('100:Master/Bot/Benchmark/Story/Metric1:100');

    const newBugBtn = (await testBed.page.$('.bucket-card .new-bug-btn'))!;
    const [newPage] = await Promise.all([
      new Promise<Page>((resolve) => testBed.page.once('popup', resolve)),
      newBugBtn.click(),
    ]);

    await newPage.waitForNavigation();
    expect(newPage.url()).to.equal('https://issues.chromium.org/issues/12345');
  });

  it('should open Existing Bug dialog when Existing Bug button is clicked', async () => {
    await triagePanelSkPO.newBucketInput.type('Existing Bug Bucket');
    await triagePanelSkPO.addBucketButton.click();

    const textarea = (await testBed.page.$('.bucket-card .bucket-textarea'))!;
    await textarea.type('100:Master/Bot/Benchmark/Story/Metric1:100');

    const existingBtn = (await testBed.page.$('.bucket-card .existing-bug-btn'))!;
    await existingBtn.click();

    expect(triagePanelSkPO.existingBugDialog.dialog).is.not.null;
  });
});

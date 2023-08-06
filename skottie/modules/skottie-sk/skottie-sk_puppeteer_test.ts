import { expect } from 'chai';
import { Page } from 'puppeteer';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('skottie-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await takeScreenshot(testBed.page, 'skottie', 'default_page');
    });

    it('shows lottie-web animation when a box is checked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#options-open');
      await testBed.page.click('checkbox-sk[label="Show lottie-web"]');
      await takeScreenshot(testBed.page, 'skottie', 'lottie_web');
      expect(testBed.page.url()).contains('l=true');
    });

    it('shows JSON editor when the button is checked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#view-json-layers');
      await takeScreenshot(testBed.page, 'skottie', 'json_editor');
      expect(testBed.page.url()).contains('e=true');
    });

    it('shows GIF exporter when the button is checked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.select('#view-exporter select', 'gif');
      const gifExporter = await testBed.page.$eval(
        '#export-form-gif',
        (ele: Element) => ele
      );
      expect(gifExporter).to.exist;
      await takeScreenshot(testBed.page, 'skottie', 'gif_exporter');
    });

    it('shows performance chart when the button is clicked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#view-perf-chart');
      await takeScreenshot(testBed.page, 'skottie', 'performance_chart');
      expect(testBed.page.url()).contains('p=true');
    });

    it('shows a dialog to upload multiple lotties when the details is expanded', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#library-open');
      await takeScreenshot(testBed.page, 'skottie', 'library_upload');
      expect(testBed.page.url()).contains('i=true');
    });

    it('shows audio uploader when a box is checked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#audio-open');
      await takeScreenshot(testBed.page, 'skottie', 'audio_upload');
      expect(testBed.page.url()).contains('a=true');
    });

    it('shows the embed code when a button is clicked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#embed-open');
      await takeScreenshot(testBed.page, 'skottie', 'embed_instructions');
    });

    it('shows the upload animation dialog when a button is clicked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('.edit-config');
      await takeScreenshot(testBed.page, 'skottie', 'upload_animation');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams: string = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, [
    'initial-animation-loaded',
  ]);
  const loaded = eventPromise('initial-animation-loaded');
  await page.goto(`${base}${queryParams}`);
  await loaded;
  // To make the animation be in the same place, we pause the animation and rewind it to the
  // beginning.
  await pause(page);
  await rewind(page);
  await page.focus('a');
}

async function pause(page: Page) {
  await page.$eval('#playpause-pause', (ele: Element) => {
    if ((ele as HTMLElement).style.display === 'inherit') {
      page.click('#playpause');
    }
  });
}

async function rewind(page: Page) {
  await page.click('#rewind');
}

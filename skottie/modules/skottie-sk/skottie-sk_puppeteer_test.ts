import { expect } from 'chai';
import { Page } from 'puppeteer';
import path from 'path';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('skottie-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed(path.join(__dirname, '..', '..', 'webpack.config.ts'));
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await takeScreenshot(testBed.page, 'skottie', 'default_page');
    });

    it('shows lottie-web animation when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show lottie-web"]');
      await takeScreenshot(testBed.page, 'skottie', 'lottie_web');
      expect(testBed.page.url()).contains('l=true');
    });

    it('shows JSON editor when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show editor"]');
      await takeScreenshot(testBed.page, 'skottie', 'json_editor');
      expect(testBed.page.url()).contains('e=true');
    });

    it('shows GIF exporter when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show gif exporter"]');
      await takeScreenshot(testBed.page, 'skottie', 'gif_exporter');
      expect(testBed.page.url()).contains('g=true');
    });

    it('shows text editor when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html?test=withText`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 1200 });
      await testBed.page.click('checkbox-sk[label="Show text editor"]');
      await takeScreenshot(testBed.page, 'skottie', 'text_editor');
      expect(testBed.page.url()).contains('t=true');
    });

    it('shows performance chart when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show performance chart"]');
      await takeScreenshot(testBed.page, 'skottie', 'performance_chart');
      expect(testBed.page.url()).contains('p=true');
    });

    it('shows a dialog to upload multiple lotties when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show library"]');
      await takeScreenshot(testBed.page, 'skottie', 'library_upload');
      expect(testBed.page.url()).contains('i=true');
    });

    it('shows audio uploader when a box is checked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('checkbox-sk[label="Show audio"]');
      await takeScreenshot(testBed.page, 'skottie', 'audio_upload');
      expect(testBed.page.url()).contains('a=true');
    });

    it('shows the embed code when a button is clicked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('#embed-btn');
      await takeScreenshot(testBed.page, 'skottie', 'embed_instructions');
    });

    it('shows the upload animation dialog when a button is clicked', async () => {
      await navigateTo(testBed.page, `${testBed.baseUrl}/static/skottie-sk.html`);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 800 });
      await testBed.page.click('.edit-config');
      await takeScreenshot(testBed.page, 'skottie', 'upload_animation');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams: string = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['initial-animation-loaded']);
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
  await page.$eval('#playpause', ((ele: Element) => {
    if (ele.textContent === 'Pause') {
      (ele as HTMLButtonElement).click();
    }
  }));
}

async function rewind(page: Page) {
  await page.click('#rewind');
}

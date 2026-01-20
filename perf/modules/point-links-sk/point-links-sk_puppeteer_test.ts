import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';

describe('point-links-sk', () => {
  let testBed: TestBed;
  let pointLinksSk: ElementHandle;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 550 });

    pointLinksSk = (await testBed.page.$('point-links-sk'))!;

    // Set displayUrls and displayTexts directly for the component.
    await pointLinksSk.evaluate(
      async (el: any, displayUrls, displayTexts) => {
        // Made async
        el.displayUrls = displayUrls;
        el.displayTexts = displayTexts;
        await el.renderPointLinks();
      },
      {
        V8: 'https://chromium.googlesource.com/v8/v8/+log/f052b8c4db1f08d1f8275351c047854e6ff1805f..47f420e89ec1b33dacc048d93e0317ab7fec43dd?n=1000',
      },
      {
        V8: 'f052b8c4 - 47f420e8',
      }
    );
  });

  afterEach(async () => {
    // No request interception or dynamic loading to clean up.
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('point-links-sk')).to.have.length(1);
  });

  it('shows the default view', async () => {
    await takeScreenshot(testBed.page, 'perf', 'point-links-sk');
  });

  it('should display the correct key and link text', async () => {
    await testBed.page.waitForSelector('#tooltip-text a', { visible: true });

    const keyText = await testBed.page.evaluate(() => {
      const keySpan = document
        .querySelector('point-links-sk')
        ?.shadowRoot?.querySelector('#tooltip-key');
      return keySpan ? keySpan.textContent?.trim() : '';
    });
    const linkText = await testBed.page.evaluate(() => {
      const linkSpan = document
        .querySelector('point-links-sk')
        ?.shadowRoot?.querySelector('#tooltip-text a');
      return linkSpan ? linkSpan.textContent?.trim() : '';
    });

    expect(keyText).to.not.null;
    expect(linkText).to.not.null;
    await takeScreenshot(testBed.page, 'point-links', 'point-links-sk');
  });
});

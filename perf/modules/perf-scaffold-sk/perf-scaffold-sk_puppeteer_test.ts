import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('perf-scaffold-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 500, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('perf-scaffold-sk')).to.have.length(1);
  });

  it('should have chrome logo', async () => {
    const logo = await testBed.page.$('.header-brand .logo');

    expect(logo).to.not.equal(null);

    const naturalWidth = await testBed.page.$eval(
      '.header-brand .logo',
      (img) => (img as HTMLImageElement).naturalWidth
    );

    expect(naturalWidth).to.be.greaterThan(0);
  });

  it('should have favicon link', async () => {
    const favicon = await testBed.page.$('link[rel="icon"]');

    expect(favicon).to.not.equal(null);

    const href = await testBed.page.$eval('link[rel="icon"]', (el) => (el as HTMLLinkElement).href);

    expect(href).to.contain('/dist/images/line-chart.svg');
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'perf-scaffold-sk');
    });
  });
});

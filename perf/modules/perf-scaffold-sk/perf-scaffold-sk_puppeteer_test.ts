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

  it('renders git hash version', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: '83cd5d7049b8b69435b93c4778235f5ce8816ac3',
        git_repo_url: 'https://repo',
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.contain('Ver: 83cd5d7');
    const href = await testBed.page.evaluate((el) => el!.getAttribute('href'), versionLink);
    expect(href).to.equal('https://repo/+/83cd5d7049b8b69435b93c4778235f5ce8816ac3');
  });

  it('renders dev timestamp version', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: 'dev-2025-11-10T21:55:47Z',
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.contain('dev-build (2025-11-10 21:55 UTC)');
  });

  it('renders fallback when app_version is missing', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: '',
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.contain('dev-build (');
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'perf-scaffold-sk');
    });
  });
});

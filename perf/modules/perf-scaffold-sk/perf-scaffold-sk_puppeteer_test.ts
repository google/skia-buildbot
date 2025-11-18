import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('perf-scaffold-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1280, height: 1024 });
  });

  it('defaults to legacy ui', async () => {
    await testBed.page.evaluate(() => localStorage.clear());
    await testBed.page.goto(testBed.baseUrl);
    const sidebar = await testBed.page.$('aside#sidebar');
    expect(sidebar).to.not.equal(null);
    const nav = await testBed.page.$('#header-nav-items');
    expect(nav).to.equal(null);
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
        enable_v2_ui: false,
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.contain('Ver: 83cd5d7');
    const href = await testBed.page.evaluate((el) => el!.getAttribute('href'), versionLink);
    expect(href).to.equal(
      'https://skia.googlesource.com/buildbot.git/+/83cd5d7049b8b69435b93c4778235f5ce8816ac3'
    );
  });

  it('renders dev timestamp version', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: 'dev-2025-11-10T21:55:47Z',
        enable_v2_ui: false,
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
        enable_v2_ui: false,
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.contain('dev-build (');
  });

  it('legacy ui shows valid version', async () => {
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
    await testBed.page.goto(testBed.baseUrl);
    const versionLink = await testBed.page.$('#links a.version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.not.contain('No Tag');
    const valid = text?.includes('Ver:') || text?.includes('dev-build');
    expect(valid).to.equal(true);
  });

  it('v2 ui shows valid version', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: '83cd5d7049b8b69435b93c4778235f5ce8816ac3',
        enable_v2_ui: true,
      };
    });
    await testBed.page.evaluate(() => {
      localStorage.setItem('v2_ui', 'true');
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.waitForSelector('app-sk.v2-ui');

    const versionLink = await testBed.page.$('.dashboard-version');
    expect(versionLink).to.not.equal(null);
    const text = await testBed.page.evaluate((el) => el!.textContent, versionLink);
    expect(text).to.not.contain('No Tag');
    expect(text).to.contain('Build:');
  });

  it('does not render V2 UI toggle when disabled', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: '',
        enable_v2_ui: false,
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    const toggle = await testBed.page.$('.try-v2-ui');
    expect(toggle).to.equal(null);
  });

  it('v2 ui shows legacy button', async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        app_version: '',
        enable_v2_ui: true,
      };
      localStorage.setItem('v2_ui', 'true');
    });
    await testBed.page.goto(testBed.baseUrl);
    // Check for a class specific to V2 UI
    const v2App = await testBed.page.$('app-sk.v2-ui');
    expect(v2App).to.not.equal(null);

    const legacyButton = await testBed.page.$('#legacy-ui-button');
    expect(legacyButton).to.not.equal(null);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'perf-scaffold-sk');
    });
  });
});

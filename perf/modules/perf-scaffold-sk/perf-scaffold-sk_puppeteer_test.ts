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

  it('legacy ui should not clip overflow', async () => {
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
    await testBed.page.goto(testBed.baseUrl);
    const overflow = await testBed.page.$eval(
      'perf-scaffold-sk',
      (el) => window.getComputedStyle(el).overflow
    );
    expect(overflow).to.not.equal('hidden');
  });

  it('legacy ui sidebar should scroll', async () => {
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
    await testBed.page.goto(testBed.baseUrl);
    const overflow = await testBed.page.$eval(
      'aside#sidebar',
      (el) => window.getComputedStyle(el).overflowY
    );
    expect(overflow).to.equal('auto');
  });

  it('v2 ui should contain overflow', async () => {
    await testBed.page.evaluate(() => {
      localStorage.setItem('v2_ui', 'true');
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.waitForSelector('app-sk.v2-ui');

    // Host should still not clip (based on fix)
    const hostOverflow = await testBed.page.$eval(
      'perf-scaffold-sk',
      (el) => window.getComputedStyle(el).overflow
    );
    expect(hostOverflow).to.not.equal('hidden');

    // App-sk should clip
    const appOverflow = await testBed.page.$eval(
      'app-sk.v2-ui',
      (el) => window.getComputedStyle(el).overflow
    );
    expect(appOverflow).to.equal('hidden');

    // Cleanup
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
  });

  it('legacy ui main content should scroll', async () => {
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
    await testBed.page.goto(testBed.baseUrl);
    const overflow = await testBed.page.$eval(
      '#perf-content',
      (el) => window.getComputedStyle(el).overflowY
    );
    expect(overflow).to.equal('auto');
  });

  it('v2 ui main content should scroll', async () => {
    await testBed.page.evaluate(() => {
      localStorage.setItem('v2_ui', 'true');
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.waitForSelector('app-sk.v2-ui');
    const overflow = await testBed.page.$eval(
      '#perf-content',
      (el) => window.getComputedStyle(el).overflowY
    );
    expect(overflow).to.equal('auto');

    // Cleanup
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
  });

  it('legacy ui shows try v2 button', async () => {
    await testBed.page.evaluate(() => {
      localStorage.removeItem('v2_ui');
    });
    await testBed.page.goto(testBed.baseUrl);
    const btn = await testBed.page.$('.try-v2-ui');
    expect(btn).to.not.equal(null);
    const visible = await testBed.page.evaluate((el) => {
      const style = window.getComputedStyle(el);
      return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
    }, btn!);
    expect(visible).to.equal(true);
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

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'perf-scaffold-sk');
    });
  });
});

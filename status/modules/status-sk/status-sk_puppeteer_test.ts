import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed
} from '../../../puppeteer-tests/util';

describe('status-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/status-sk.html`);
    await testBed.page.setViewport({ width: 1600, height: 700 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('status-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'status-sk');
    });
    it('changes repos', async () => {
      testBed.page.select('#repoSelector', 'infra');
      await takeScreenshot(testBed.page, 'status', 'status-sk_repo_change');
    });
  });
});

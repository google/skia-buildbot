import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('alogin-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(path.join(__dirname, '..', '..', 'webpack.config.ts'));
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 600 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('alogin-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'alogin-sk');
    });
  });
});

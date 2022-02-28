import { expect } from 'chai';
import {
  inBazel, loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('note-editor-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/note-editor-sk.html`);
    await testBed.page.setViewport({ width: 640, height: 480 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('note-editor-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'machine', 'note-editor-sk');
    });
  });
});

import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('icons-demo-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setViewport({ width: 1300, height: 1500 });
    await testBed.page.goto(testBed.baseUrl);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('icons-demo-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    const categories = [
      'action',
      'alert',
      'av',
      'communication',
      'content',
      'device',
      'editor',
      'file',
      'hardware',
      'image',
      'maps',
      'navigation',
      'notification',
      'places',
      'social',
      'toggle',
    ];

    categories.forEach((category: string) => {
      it(`shows icons from category ${category}`, async () => {
        const categoryIcons = await testBed.page.$(`.category-${category}`);
        await takeScreenshot(categoryIcons!, 'elements-sk', `icons-demo-sk_${category}`);
      });
    });
  });
});

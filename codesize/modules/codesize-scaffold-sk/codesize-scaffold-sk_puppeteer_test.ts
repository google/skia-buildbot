import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('codesize-scaffold-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('codesize-scaffold-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('uses darkmode theme by default', async () => {
      // Defaults to dark mode.
      await expectDarkMode();
      await takeScreenshot(testBed.page, 'codesize', 'codesize-scaffold-sk_darkmode');
    });

    it('switches to non-darkmode theme when clicking the theme chooser', async () => {
      await testBed.page.click('theme-chooser-sk');
      await expectNonDarkMode();
      await takeScreenshot(testBed.page, 'codesize', 'codesize-scaffold-sk');
      await testBed.page.click('theme-chooser-sk');
      await expectDarkMode();
    });
  });

  const expectBodyClassName = async (className: string) => {
    const body = await testBed.page.$('body');
    const classProp = await body.getProperty('className');
    expect(await classProp.jsonValue()).to.equal(className);
  };

  const expectDarkMode = () => expectBodyClassName('body-sk darkmode');
  const expectNonDarkMode = () => expectBodyClassName('body-sk');
});

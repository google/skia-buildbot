import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage, inBazel,
  loadCachedTestBed,
  takeScreenshot,
  TestBed
} from '../../../puppeteer-tests/util';
import path from "path";

describe('byblame-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
    const loaded = eventPromise('end-task'); // Emitted when page is loaded.
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/byblame-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('byblame-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 2700 });
    await takeScreenshot(testBed.page, 'gold', 'byblame-page-sk');
  });

  it('limits the number of commits on the blamelist', async () => {
    await testBed.page.setViewport({ width: 1200, height: 2300 });
    // click on "svg"
    await testBed.page.click('corpus-selector-sk > ul > li:nth-child(3)');
    await expectSelectedCorpusToBe('svg');
    await takeScreenshot(testBed.page, 'gold', 'byblame-page-sk_limits-blamelist-commits');
  });

  it('responds to forward and back browser buttons', async () => {
    await expectSelectedCorpusToBe('gm');

    // click on canvaskit
    await testBed.page.click('corpus-selector-sk > ul > li:nth-child(1)');
    await expectSelectedCorpusToBe('canvaskit');
    expect(testBed.page.url()).to.contain('?corpus=canvaskit');

    // click on "svg"
    await testBed.page.click('corpus-selector-sk > ul > li:nth-child(3)');
    await expectSelectedCorpusToBe('svg');
    expect(testBed.page.url()).to.contain('?corpus=svg');

    await testBed.page.goBack();
    await expectSelectedCorpusToBe('canvaskit');
    expect(testBed.page.url()).to.contain('?corpus=canvaskit');

    await testBed.page.goForward();
    await expectSelectedCorpusToBe('svg');
    expect(testBed.page.url()).to.contain('?corpus=svg');
  });

  const expectSelectedCorpusToBe = async (corpus: string) => {
    const selectedTitle =
        await testBed.page.$eval(
            'corpus-selector-sk li.selected',
            (e: Element) => (e as HTMLLIElement).innerText);
    expect(selectedTitle).to.contain(corpus);
  };
});

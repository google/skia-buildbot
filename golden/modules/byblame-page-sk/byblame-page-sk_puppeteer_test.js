const expect = require('chai').expect;
const path = require('path');
const addEventListenersToPuppeteerPage = require('../../../puppeteer-tests/util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('../../../puppeteer-tests/util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('../../../puppeteer-tests/util').takeScreenshot;

describe('byblame-page-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
    const loaded = eventPromise('end-task'); // Emitted when page is loaded.
    await testBed.page.goto(`${testBed.baseUrl}/dist/byblame-page-sk.html`);
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

  it('responds to forward and back browser buttons', async () => {
    const expectSelectedCorpusToBe = async (corpus) => {
      const selectedTitle = await testBed.page.$eval('corpus-selector-sk li.selected', (e) => e.innerText);
      expect(selectedTitle).to.contain(corpus);
    };

    await expectSelectedCorpusToBe('gm');

    // click on canvaskit
    await testBed.page.click('corpus-selector-sk > ul > li:nth-child(1)');
    await expectSelectedCorpusToBe('canvaskit');
    expect(testBed.page.url()).to.contain('?corpus=canvaskit');

    // click on svg
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
});

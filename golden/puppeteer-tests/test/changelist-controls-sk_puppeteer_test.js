const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('changelist-controls-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/changelist-controls-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('changelist-controls-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    const controls = await testBed.page.$('.search_response');
    await takeScreenshot(controls, 'changelist-controls-sk');
  });
});

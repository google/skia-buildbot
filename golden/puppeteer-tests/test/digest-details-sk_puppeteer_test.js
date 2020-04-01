const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('digest-details-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/digest-details-sk.html`, { waitUntil: 'networkidle0' });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('digest-details-sk')).to.have.length(6);
  });

  describe('screenshots', () => {
    it('has the left and right image', async () => {
      const digestDetailsSk = await testBed.page.$('#normal');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk');
    });

    it('was given data with only a negative image to compare against', async () => {
      const digestDetailsSk = await testBed.page.$('#negative_only');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk_negative-only');
    });

    it('was given data no other images to compare against', async () => {
      const digestDetailsSk = await testBed.page.$('#no_refs');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk_no-refs');
    });

    it('was given a changelist id', async () => {
      const digestDetailsSk = await testBed.page.$('#changelist_id');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk_changelist-id');
    });

    it('had the right side overridden', async () => {
      const digestDetailsSk = await testBed.page.$('#right_overridden');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk_right-overridden');
    });

    it('had no trace data sent by the backend', async () => {
      const digestDetailsSk = await testBed.page.$('#no_traces');
      await takeScreenshot(digestDetailsSk, 'digest-details-sk_no-traces');
    });
  });
});

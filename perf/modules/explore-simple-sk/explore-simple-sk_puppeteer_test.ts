import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { ExploreSimpleSkPO } from './explore-simple-sk_po';
import { ParamSet } from '../../../infra-sk/modules/query';
import { DEFAULT_VIEWPORT } from '../common/puppeteer-test-util';

describe('explore-simple-sk', () => {
  let testBed: TestBed;
  let exploreSimpleSk: ElementHandle;
  let simplePageSkPO: ExploreSimpleSkPO;

  const query: ParamSet = {
    arch: ['arm', 'x86'],
    config: ['android'],
    compiler: ['~CC'],
  };

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport(DEFAULT_VIEWPORT);
    exploreSimpleSk = (await testBed.page.$('explore-simple-sk'))!;
    simplePageSkPO = new ExploreSimpleSkPO(exploreSimpleSk);

    // Make the buttons visible for the tests
    await testBed.page.evaluate((el: Element) => {
      if (el) {
        const buttons = el.querySelector<HTMLElement>('#buttons');
        if (buttons) {
          buttons.style.display = 'block';
        }
      }
    }, exploreSimpleSk);
  });

  afterEach(async () => {
    await testBed.page.setRequestInterception(false);
    testBed.page.removeAllListeners('request');
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('explore-simple-sk')).to.have.length(2); // Smoke test.
  });

  describe('query dialog interaction', () => {
    it('should have the query dialog element in the page on initial load', async () => {
      const dialog = await testBed.page.evaluate((el: Element) => {
        return el.shadowRoot?.querySelector('dialog#query-dialog');
      }, exploreSimpleSk);
      expect(dialog).to.not.be.null;
    });

    it('should have the query-sk element in the shadow DOM on initial load', async () => {
      const querySk = await testBed.page.evaluate((el: Element) => {
        return el.shadowRoot?.querySelector('query-sk');
      }, exploreSimpleSk);
      expect(querySk).to.not.be.null;
    });
  });

  describe('query-sk interactions', () => {
    it('should display initial query from state', async () => {
      const querySkPO = await simplePageSkPO.querySk;
      const initialValue = await querySkPO.getCurrentQuery();
      expect(initialValue).is.not.null;
    });

    it('should allow updating the query', async () => {
      const querySkPO = await simplePageSkPO.querySk;

      await querySkPO.setCurrentQuery(query);
      const currentValue = await querySkPO.getCurrentQuery();
      expect(currentValue.config).not.null;
      expect(currentValue.arch).not.null;
      expect(currentValue.compiler).not.null;
    });
  });

  describe('plot-google-chart-sk display', () => {
    it('should not render the chart on initial load', async () => {
      await testBed.page.waitForSelector('explore-simple-sk');
      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      expect(await plotPO.isChartVisible()).to.be.false;
    });
  });
});

import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ChartTooltipSkPO } from './chart-tooltip-sk_po';
import { ElementHandle } from 'puppeteer';
import { assert } from 'chai';

describe('chart-tooltip-sk', () => {
  let testBed: TestBed;
  let chartTooltipSkPO: ChartTooltipSkPO;
  let chartTooltipSk: ElementHandle;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
    chartTooltipSk = (await testBed.page.$('chart-tooltip-sk'))!;
    if (!chartTooltipSk) {
      throw new Error('chart-tooltip-sk not found');
    }
    chartTooltipSkPO = new ChartTooltipSkPO(chartTooltipSk);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('chart-tooltip-sk')).to.have.length(1);
  });

  describe('populate data', () => {
    beforeEach(async () => {
      await testBed.page.click('#reset-tooltip');
    });

    it('loads data without anomaly', async () => {
      await testBed.page.click('#load-data-without-anomaly');

      const anomalyDetails = await chartTooltipSkPO.anomalyDetails;
      assert.isTrue(await (await anomalyDetails).isEmpty());

      expect(await chartTooltipSkPO.title.innerText).to.contain(
        'ChromiumPerf/win-10-perf/jetstream2/stanford-crypto-aes.Average/JetStream2'
      );

      const table = chartTooltipSkPO.table;
      const rows = await table.bySelectorAll('li');
      expect(await rows.length).to.equal(3);

      const dateRow = await rows.item(0);
      expect(await dateRow.innerText).to.contain('Date');
      expect(await dateRow.innerText).to.contain('Wed, 15 Mar 2023 13:20:00 GMT');

      const valueRow = await rows.item(1);
      expect(await valueRow.innerText).to.contain('Value');
      expect(await valueRow.innerText).to.contain('100 ms');

      const pointRangeRow = await rows.item(2);
      expect(await pointRangeRow.innerText).to.contain('Change');
      expect(await pointRangeRow.innerText).to.contain('12346');

      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-load-without-anomaly');
    });

    it('loads anomaly data', async () => {
      await testBed.page.click('#load-data-with-anomaly');

      expect(await chartTooltipSkPO.title.innerText).to.contain(
        'ChromiumPerf/win-10-perf/jetstream2/stanford-crypto-aes.Average/JetStream2 [Anomaly]'
      );

      // Don't check other point metadata, as it's tested by other tests cases.
      // Only check the anomaly details.

      const anomalyDetails = await chartTooltipSkPO.anomalyDetails;
      assert.isNotEmpty(await anomalyDetails);

      const rows = await anomalyDetails.bySelectorAll('li');
      expect(await rows.length).to.equal(5);

      const anomalyRow = await rows.item(0);
      expect(await anomalyRow.innerText).to.contain('Anomaly');
      expect(await anomalyRow.innerText).to.contain('Regression');

      const medianRow = await rows.item(1);
      expect(await medianRow.innerText).to.contain('Median');
      expect(await medianRow.innerText).to.contain('100.5023 ms');

      const previousRow = await rows.item(2);
      expect(await previousRow.innerText).to.contain('Previous');
      expect(await previousRow.innerText).to.contain('75.2091 [+33.6305%]');

      const bugIdRow = await rows.item(3);
      expect(await bugIdRow.innerText).to.contain('Bug ID');
      expect(await bugIdRow.innerText).to.contain('12345');

      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-load-anomaly-details');
    });
  });
});

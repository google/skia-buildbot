import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { TestPickerSkPO } from './test-picker-sk_po';
import { DEFAULT_VIEWPORT } from '../common/puppeteer-test-util';

const BENCHMARK = 'blink_perf.css';
const BOT = 'ToTLinuxTSan';
const TEST =
  'memory:chrome:renderer_processes:reported_by_chrome:v8:heap:code_space:effective_size_max';
const SUBTEST_1 = 'link_invalidation_document_rules.html';
const SUBTEST_2 = 'AdsAdSenseAsyncAds_warm';

const TEST_NEW = 'motion_mark_canvas_fill_shapes';
const SUBTEST_1_NEW = 'line-layout.html';
const SUBTEST_2_NEW = 'AdsAdSenseAsyncAds_cold';

describe('test-picker-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport(DEFAULT_VIEWPORT);
  });

  it('should render the component', async () => {
    await testBed.page.waitForSelector('test-picker-sk');
  });

  it('selects items one by one and verifies the query', async () => {
    const testPickerPO = new TestPickerSkPO((await testBed.page.$('test-picker-sk'))!);

    // Wait for the first field to be available.
    await testPickerPO.waitForPickerField(0);
    const benchmarkField = await testPickerPO.getPickerField(0);
    // 'blink_perf.css' is a valid option.
    await benchmarkField.search(BENCHMARK);
    await testPickerPO.waitForSpinnerInactive();
    await takeScreenshot(testBed.page, 'perf', 'test-picker-sk-benchmark-selected');

    // Wait for the next field to appear (Bot).
    await testPickerPO.waitForPickerField(1);
    const botField = await testPickerPO.getPickerField(1);
    await botField.search(BOT);
    await testPickerPO.waitForSpinnerInactive();

    // Wait for the next field (Test).
    await testPickerPO.waitForPickerField(2);
    const testField = await testPickerPO.getPickerField(2);
    await testField.search(TEST);
    await testPickerPO.waitForSpinnerInactive();

    // Wait for the next field (Subtest1).
    await testPickerPO.waitForPickerField(3);
    const subtest1Field = await testPickerPO.getPickerField(3);
    await subtest1Field.search(SUBTEST_1);
    await testPickerPO.waitForSpinnerInactive();

    // Wait for the next field (Subtest2).
    await testPickerPO.waitForPickerField(4);
    const subtest2Field = await testPickerPO.getPickerField(4);
    await subtest2Field.search(SUBTEST_2);
    await testPickerPO.waitForSpinnerInactive();

    // Click the plot button.
    await testPickerPO.clickPlotButton();

    // Verify the query event.
    // In the demo page, the event detail is dumped into <pre id="events">.
    const eventsPre = (await testBed.page.$('#events'))!;
    await testBed.page.waitForFunction(
      (el) => el.textContent && el.textContent.length > 0,
      {},
      eventsPre
    );
    const query = await eventsPre.evaluate((el) => el.textContent);

    const expectedQuery = [
      `benchmark=${BENCHMARK}`,
      `&bot=${BOT}`,
      `&subtest1=${SUBTEST_1}`,
      `&subtest2=${SUBTEST_2}`,
      `&test=${encodeURIComponent(TEST)}`,
    ].join('');

    expect(query).to.equal(expectedQuery);
  });

  it('selects all, deletes middle, and refills with another path', async () => {
    const testPickerPO = new TestPickerSkPO((await testBed.page.$('test-picker-sk'))!);

    // 1. Fill all selectors
    // Benchmark
    await testPickerPO.waitForPickerField(0);
    const benchmarkField = await testPickerPO.getPickerField(0);
    await benchmarkField.search(BENCHMARK);
    await testPickerPO.waitForSpinnerInactive();

    // Bot
    await testPickerPO.waitForPickerField(1);
    const botField = await testPickerPO.getPickerField(1);
    await botField.search(BOT);
    await testPickerPO.waitForSpinnerInactive();

    // Test
    await testPickerPO.waitForPickerField(2);
    const testField = await testPickerPO.getPickerField(2);
    await testField.search(TEST);
    await testPickerPO.waitForSpinnerInactive();

    // Subtest1
    await testPickerPO.waitForPickerField(3);
    const subtest1Field = await testPickerPO.getPickerField(3);
    await subtest1Field.search(SUBTEST_1);
    await testPickerPO.waitForSpinnerInactive();

    // Subtest2
    await testPickerPO.waitForPickerField(4);
    const subtest2Field = await testPickerPO.getPickerField(4);
    await subtest2Field.search(SUBTEST_2);
    await testPickerPO.waitForSpinnerInactive();

    // 2. Delete in the middle (Test field)
    await testField.clear();
    await testPickerPO.waitForSpinnerInactive();

    // 3. Refill with another path
    // Refill Test
    await testField.search(TEST_NEW);
    await testPickerPO.waitForSpinnerInactive();

    // Refill Subtest1
    await testPickerPO.waitForPickerField(3);
    const subtest1FieldNew = await testPickerPO.getPickerField(3);
    await subtest1FieldNew.search(SUBTEST_1_NEW);
    await testPickerPO.waitForSpinnerInactive();

    // Refill Subtest2
    await testPickerPO.waitForPickerField(4);
    const subtest2FieldNew = await testPickerPO.getPickerField(4);
    await subtest2FieldNew.search(SUBTEST_2_NEW);
    await testPickerPO.waitForSpinnerInactive();

    // Click plot
    await testPickerPO.clickPlotButton();

    // Verify
    const eventsPre = (await testBed.page.$('#events'))!;
    await testBed.page.waitForFunction(
      (el) => el.textContent && el.textContent.length > 0,
      {},
      eventsPre
    );
    const query = await eventsPre.evaluate((el) => el.textContent);
    const expectedQuery = [
      `benchmark=${BENCHMARK}`,
      `&bot=${BOT}`,
      `&subtest1=${SUBTEST_1_NEW}`,
      `&subtest2=${SUBTEST_2_NEW}`,
      `&test=${encodeURIComponent(TEST_NEW)}`,
    ].join('');
    expect(query).to.equal(expectedQuery);
  });
});

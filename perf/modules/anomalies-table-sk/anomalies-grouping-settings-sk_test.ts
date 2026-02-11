import './anomalies-grouping-settings-sk';
import { assert } from 'chai';
import { AnomaliesGroupingSettingsSk } from './anomalies-grouping-settings-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { AnomalyGroupingConfig } from './grouping';

describe('anomalies-grouping-settings-sk', () => {
  const newInstance = setUpElementUnderTest<AnomaliesGroupingSettingsSk>(
    'anomalies-grouping-settings-sk'
  );

  let element: AnomaliesGroupingSettingsSk;
  const defaultConfig: AnomalyGroupingConfig = {
    revisionMode: 'OVERLAPPING',
    groupBy: new Set(),
    groupSingles: false,
  };

  beforeEach(() => {
    element = newInstance();
    element.config = { ...defaultConfig };
  });

  it('renders with default config', async () => {
    await element.updateComplete;
    const select = element.querySelector<HTMLSelectElement>('select[id^="revision-mode-select"]');
    assert.isNotNull(select);
    assert.equal(select!.value, 'OVERLAPPING');
  });

  it('uses uniqueId in element IDs', async () => {
    element.uniqueId = 'test-uid';
    // We need to wait for the element to re-render after property change
    await element.requestUpdate();
    await element.updateComplete;

    const select = element.querySelector('#revision-mode-select-test-uid');
    assert.isNotNull(select, 'Select element with unique ID should exist');
    const label = element.querySelector('label[for="revision-mode-select-test-uid"]');
    assert.isNotNull(label, 'Label with correct for attribute should exist');
  });

  it('dispatches revision-mode-change event', async () => {
    await element.updateComplete;
    let eventDetail: any;
    element.addEventListener('revision-mode-change', (e: any) => {
      eventDetail = e.detail;
    });

    const select = element.querySelector<HTMLSelectElement>('select[id^="revision-mode-select"]')!;
    select.value = 'EXACT';
    select.dispatchEvent(new Event('change', { bubbles: true }));

    assert.equal(eventDetail, 'EXACT');
  });

  it('dispatches group-singles-change event', async () => {
    await element.updateComplete;
    let eventDetail: any;
    element.addEventListener('group-singles-change', (e: any) => {
      eventDetail = e.detail;
    });

    // The first checkbox in the second group is for single anomalies
    const groups = element.querySelectorAll('.grouping-setting-group');
    const singlesGroup = groups[1];
    const singlesCheckbox = singlesGroup.querySelector(
      'input[type="checkbox"]'
    ) as HTMLInputElement;

    singlesCheckbox.checked = true;
    singlesCheckbox.dispatchEvent(new Event('change', { bubbles: true }));

    assert.isTrue(eventDetail);
  });

  it('dispatches group-by-change event', async () => {
    await element.updateComplete;
    let eventDetail: any;
    element.addEventListener('group-by-change', (e: any) => {
      eventDetail = e.detail;
    });

    // Find the BENCHMARK checkbox (first in Split Groups By)
    const groups = element.querySelectorAll('.grouping-setting-group');
    const splitGroup = groups[2];
    const benchmarkCheckbox = splitGroup.querySelector(
      'input[value="BENCHMARK"]'
    ) as HTMLInputElement;

    benchmarkCheckbox.checked = true;
    benchmarkCheckbox.dispatchEvent(new Event('change', { bubbles: true }));

    assert.deepEqual(eventDetail, { criteria: 'BENCHMARK', enabled: true });
  });
});

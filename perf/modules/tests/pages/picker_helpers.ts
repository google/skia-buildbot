import { Page } from 'playwright-core';

/**
 * Shared interaction helpers for test-picker-sk and picker-field-sk on /m and /e.
 */
export async function selectPickerField(
  page: Page,
  pickerSelector: string,
  label: string,
  values: string[]
): Promise<void> {
  const fieldLocator = page.locator(
    `${pickerSelector} picker-field-sk[label="${label}"]:not([disabled])`
  );
  await fieldLocator.waitFor({ state: 'visible', timeout: 10000 });
  const comboLocator = fieldLocator.locator('vaadin-multi-select-combo-box:not([readonly])');
  await comboLocator.waitFor({ state: 'visible', timeout: 10000 });
  await fieldLocator.evaluate((el: any, items: string[]) => {
    if (el.disabled) {
      throw new Error(`picker-field-sk[label="${el.label}"] is disabled`);
    }
    const combo = el.querySelector('vaadin-multi-select-combo-box') as any;
    if (!combo || combo.readonly) {
      throw new Error(`vaadin-multi-select-combo-box in "${el.label}" is missing or readonly`);
    }
    combo.opened = false;
    combo.selectedItems = items;
    combo.dispatchEvent(
      new CustomEvent('selected-items-changed', {
        detail: { value: items },
        bubbles: true,
        composed: true,
      })
    );
  }, values);
  await page.waitForFunction((sel: string) => {
    const tp = document.querySelector(sel) as any;
    return tp && !tp.ctrl?.requestInProgress;
  }, pickerSelector);
  await page.evaluate(() => {
    document.querySelectorAll('vaadin-multi-select-combo-box').forEach((c: any) => {
      c.opened = false;
    });
  });
}

/**
 * Sequentially selects multiple picker-field-sk parameters in test-picker-sk.
 */
export async function selectPickerFields(
  page: Page,
  pickerSelector: string,
  fields: Record<string, string[]>
): Promise<void> {
  for (const [label, values] of Object.entries(fields)) {
    await selectPickerField(page, pickerSelector, label, values);
  }
}

/**
 * Checks or unchecks the "All" checkbox (checkbox-sk#select-all) on a
 * picker-field-sk parameter and waits for test-picker-sk to finish updating.
 */
export async function selectAllPickerField(
  page: Page,
  pickerSelector: string,
  label: string,
  checked: boolean = true
): Promise<string[]> {
  const fieldLocator = page.locator(
    `${pickerSelector} picker-field-sk[label="${label}"]:not([disabled])`
  );
  await fieldLocator.waitFor({ state: 'visible', timeout: 10000 });
  const selectAllLocator = fieldLocator.locator('checkbox-sk#select-all:not([disabled])');
  await selectAllLocator.waitFor({ state: 'attached', timeout: 10000 });
  const selected = await fieldLocator.evaluate(async (el: any, wantChecked: boolean) => {
    const allBox = el.querySelector('checkbox-sk#select-all') as any;
    if (!allBox || allBox.disabled || el.disabled) {
      throw new Error(`checkbox-sk#select-all in "${el.label}" is missing or disabled`);
    }
    const origOptions = el.options;
    el.options = ['__opt1__', '__opt2__', '__opt3__'];
    const canShowAll = Boolean(el.showSelectAll);
    el.options = origOptions;
    if (el.index > 0 && !canShowAll) {
      throw new Error(`picker-field-sk[label="${el.label}"].showSelectAll returned false`);
    }
    if (Boolean(allBox.checked) !== wantChecked) {
      const input = allBox.querySelector('input') as HTMLInputElement | null;
      if (!input || input.disabled) {
        throw new Error(`checkbox-sk#select-all input in "${el.label}" is missing or disabled`);
      }
      input.click();
    }
    if (el.updateComplete) {
      await el.updateComplete;
    }
    const combo = el.querySelector('vaadin-multi-select-combo-box') as any;
    if (el._checkboxSelected && combo) {
      combo.selectedItems = el.selectedItems;
      combo.dispatchEvent(
        new CustomEvent('selected-items-changed', {
          detail: { value: el.selectedItems },
          bubbles: true,
          composed: true,
        })
      );
    }
    return (el.selectedItems || []) as string[];
  }, checked);
  await page.waitForFunction((sel: string) => {
    const tp = document.querySelector(sel) as any;
    const em = document.querySelector('explore-multi-sk') as any;
    return tp && !tp.ctrl?.requestInProgress && (!em || !em._dataLoading);
  }, pickerSelector);
  await page.waitForLoadState('networkidle');
  return selected;
}

/**
 * Removes a single value from a picker-field-sk parameter via test-picker-sk.
 */
export async function removePickerFieldValue(
  page: Page,
  pickerSelector: string,
  label: string,
  valueToRemove: string
): Promise<void> {
  await page.evaluate(
    ({ sel, param, val }) => {
      const tp = document.querySelector(sel) as any;
      if (tp && typeof tp.removeItemFromChart === 'function') {
        tp.removeItemFromChart(param, [val]);
      }
    },
    { sel: pickerSelector, param: label, val: valueToRemove }
  );
  await page.waitForFunction((sel: string) => {
    const tp = document.querySelector(sel) as any;
    const em = document.querySelector('explore-multi-sk') as any;
    return tp && !tp.ctrl?.requestInProgress && (!em || !em._dataLoading);
  }, pickerSelector);
  await page.waitForLoadState('networkidle');
}

/**
 * Closes any open Vaadin combo boxes, clicks #plot-button in test-picker-sk,
 * and waits for network idle.
 */
export async function clickPickerPlot(page: Page, pickerSelector: string): Promise<void> {
  await page.evaluate(() => {
    document.querySelectorAll('vaadin-multi-select-combo-box').forEach((c: any) => {
      c.opened = false;
    });
  });
  const plotBtn = page.locator(`${pickerSelector} #plot-button:not([disabled])`);
  await plotBtn.waitFor({ state: 'visible', timeout: 10000 });
  await plotBtn.evaluate((btn: HTMLButtonElement) => btn.click());
  await page.waitForLoadState('networkidle');
}

import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';

export class AnomaliesTableSkPO extends PageObject {
  get rows(): PageObjectElementList {
    return this.bySelectorAll('tbody tr');
  }

  get childRows(): PageObjectElementList {
    return this.bySelectorAll('.child-expanded-row');
  }

  get parentExpandRow(): PageObjectElementList {
    return this.bySelectorAll('.parent-expanded-row');
  }

  get testPaths(): PageObjectElementList {
    return this.bySelectorAll('tbody tr td:nth-child(5)');
  }

  get bugLinks(): PageObjectElementList {
    return this.bySelectorAll('tbody tr td:nth-child(4) a');
  }

  get multiChartUrls(): PageObjectElementList {
    return this.bySelectorAll('tbody tr td:nth-child(3) a');
  }

  get triageButton(): PageObjectElement {
    return this.bySelector('[id^="triage-button-"]');
  }

  get openReport(): PageObjectElement {
    return this.bySelector('[id^="graph-button-"]');
  }

  get expandButton(): PageObjectElementList {
    return this.bySelectorAll('button.expand-button');
  }

  get headerCheckbox(): PageObjectElement {
    return this.bySelector('[id^="header-checkbox-"]');
  }

  get checkboxes(): PageObjectElementList {
    return this.bySelectorAll('tbody input[type="checkbox"]');
  }

  get trendingIconLink(): PageObjectElementList {
    return this.bySelectorAll('button#trendingicon-link');
  }

  get groupingSettingsDetails(): PageObjectElement {
    return this.bySelector('details.grouping-settings');
  }

  async getBugId(row: PageObjectElement): Promise<string> {
    const link = await row.bySelector('td:nth-child(4) a');
    return await (link?.innerText || '');
  }

  async clickTriageButton(): Promise<void> {
    await this.triageButton.click();
  }

  async clickGraphButton(): Promise<void> {
    await this.openReport.click();
  }

  async getRowCount(): Promise<number> {
    return await (
      await this.rows
    ).length;
  }

  async getParentExpandRowCount(): Promise<number> {
    return await (
      await this.parentExpandRow
    ).length;
  }

  async getChildRowCount(): Promise<number> {
    return await (
      await this.childRows
    ).length;
  }

  async getTestPath(): Promise<string[]> {
    return await this.testPaths.map(async (testPath) => await testPath.innerText);
  }

  async clickTrendingIconButton(index: number): Promise<void> {
    const trendingLinkList = await this.trendingIconLink;
    const trendingLink = trendingLinkList.item(index);
    await this.clickExpandButton(index);
    await (await trendingLink).click();
  }

  async clickExpandButton(index: number): Promise<void> {
    const expandButtons = await this.expandButton;
    const expandButton = expandButtons.item(index);
    await (await expandButton).click();
  }

  async clickHeaderCheckbox(): Promise<void> {
    await this.headerCheckbox.click();
  }

  async clickCheckbox(index: number): Promise<void> {
    const checkboxes = await this.checkboxes;
    const checkbox = checkboxes.item(index);
    await (await checkbox).click();
  }

  async clickMultiChartUrl(index: number): Promise<void> {
    const links = await this.multiChartUrls;
    const link = links.item(index);
    await (await link).click();
  }

  async isRowHidden(index: number): Promise<boolean> {
    const rows = await this.rows;
    const row = await rows.item(index);
    await row.applyFnToDOMNode((el) => el.outerHTML);
    return await row.isHidden();
  }

  async toggleGroupingSettings(shouldBeOpen: boolean): Promise<void> {
    const details = this.groupingSettingsDetails;
    const isOpen = await details.hasAttribute('open');

    // Only click if the current state is different from the desired state
    if (isOpen !== shouldBeOpen) {
      const summary = await details.bySelector('summary');
      await summary.click();
    }
  }

  async setRevisionMode(mode: 'EXACT' | 'OVERLAPPING' | 'ANY'): Promise<void> {
    await this.toggleGroupingSettings(true); // Open

    const select = await this.groupingSettingsDetails.bySelector(
      'select[id^="revision-mode-select-"]'
    );

    await select.applyFnToDOMNode((el, mode) => {
      const selectEl = el as HTMLSelectElement;
      if (selectEl.value !== mode) {
        selectEl.value = mode as string;
        selectEl.dispatchEvent(new Event('change', { bubbles: true, composed: true }));
      }
    }, mode);

    await this.toggleGroupingSettings(false); // Close
  }

  async setGroupBy(criteria: 'BENCHMARK' | 'BOT' | 'TEST', checked: boolean): Promise<void> {
    await this.toggleGroupingSettings(true); // Open

    const checkbox = await this.groupingSettingsDetails.bySelector(`input[value="${criteria}"]`);
    const isChecked = await checkbox.applyFnToDOMNode((el) => (el as HTMLInputElement).checked);
    if (isChecked !== checked) {
      await checkbox.click();
    }

    await this.toggleGroupingSettings(false); // Close
  }

  async setGroupSingles(checked: boolean): Promise<void> {
    await this.toggleGroupingSettings(true); // Open

    const checkbox = await this.groupingSettingsDetails.bySelector(
      'input[type="checkbox"]:not([value])'
    );
    const isChecked = await checkbox.applyFnToDOMNode((el) => (el as HTMLInputElement).checked);
    if (isChecked !== checked) {
      await checkbox.click();
    }

    await this.toggleGroupingSettings(false); // Close
  }
}

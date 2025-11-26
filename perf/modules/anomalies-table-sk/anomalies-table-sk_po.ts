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

  async getBugId(row: PageObjectElement): Promise<string> {
    const link = await row.bySelector('td:nth-child(4) a');
    return link?.innerText || '';
  }

  async clickTriageButton(): Promise<void> {
    await this.triageButton.click();
  }

  async clickGraphButton(): Promise<void> {
    await this.openReport.click();
  }

  async getRowCount(): Promise<number> {
    return (await this.rows).length;
  }

  async getParentExpandRowCount(): Promise<number> {
    return (await this.parentExpandRow).length;
  }

  async getChildRowCount(): Promise<number> {
    return (await this.childRows).length;
  }

  async getTestPath(): Promise<string[]> {
    return await this.testPaths.map((testPath) => testPath.innerText);
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
    return row.isHidden();
  }
}

import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';

export class PivotTableSkPO extends PageObject {
  get sortIcons(): PageObjectElementList {
    return this.bySelectorAll('div.querydef + table th > *:first-child');
  }

  async clickSortIcon(index: number): Promise<void> {
    const icon = await this.sortIcons.item(index);
    await icon.click();
  }

  get tableRows(): PageObjectElementList {
    // The browser will create a tbody.
    return this.bySelectorAll('div.querydef + table > tbody > tr');
  }

  async getCell(rowIndex: number, colIndex: number): Promise<PageObjectElement> {
    const row = await this.tableRows.item(rowIndex);
    // Row cells are a mix of th and td.
    const cells = row.bySelectorAll('th, td');
    return await cells.item(colIndex);
  }

  async getCellValue(rowIndex: number, colIndex: number): Promise<string> {
    const cell = await this.getCell(rowIndex, colIndex);
    return await cell.innerText;
  }

  async getColumnValues(colIndex: number): Promise<string[]> {
    const numRows = await this.tableRows.length;
    const values: string[] = [];
    // Start at 1 to skip the header row.
    for (let i = 1; i < numRows; i++) {
      const row = await this.tableRows.item(i);
      const cells = row.bySelectorAll('th, td');
      const cell = await cells.item(colIndex);
      values.push(await cell.innerText);
    }
    return values;
  }
}

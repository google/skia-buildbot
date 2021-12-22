/**
 * @module modules/pivot-table-sk
 * @description <h2><code>pivot-table-sk</code></h2>
 *
 * Displays a DataFrame that has been pivoted and contains summary values (as
 * opposed to a DataFrame that has been pivoted and contains summary traces).
 * These values are displayed in a table, as opposed to being displayed on in a
 * plot.
 *
 * The inputs required are a DataFrame and a pivot.Request, which has details on
 * how the input DataFrame was pivoted.
 *
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { pivot, DataFrame } from '../json';
import { validateAsPivotTable } from '../pivotutil';

import 'elements-sk/icon/sort-icon-sk';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';

type direction = 'up' | 'down';
export class PivotTableSk extends ElementSk {
  private df: DataFrame | null = null;

  private req: pivot.Request | null = null;

  /** The index into the trace values to sort rows by, with -1 sorting by the
   * row id. */
  private sortBy: number = -1;

  private sortDirection: direction = 'up';

  constructor() {
    super(PivotTableSk.template);
  }

  private static template = (ele: PivotTableSk) => {
    const invalidMessage = validateAsPivotTable(ele.req);
    if (invalidMessage) {
      return html`<h2>Cannot display: ${invalidMessage}</h2>`;
    }
    if (!ele.df) {
      return html`<h2>Cannot display: Data is missing.</h2>`;
    }
    return html`<table>
    ${ele.tableHeader()}
    ${ele.tableRows()}
  </table>`;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  set(df: DataFrame, req: pivot.Request): void {
    this.df = df;
    this.req = req;
    this._render();
  }

  private tableHeader(): TemplateResult {
    return html`
    <tr>
      <th>${this.sortArrow(-1)} Group</th>
      ${this.req!.summary!.map((summaryOperation, index) => html`<th>${this.sortArrow(index)} ${summaryOperation}</th>`)}
    </tr>`;
  }

  private sortArrow(index: number): TemplateResult {
    if (index === this.sortBy) {
      if (this.sortDirection === 'up') {
        return html`<arrow-drop-up-icon-sk title="Change sort order to descending." @click=${() => this.changeSort(index)}></arrow-drop-up-icon-sk>`;
      }
      return html`<arrow-drop-down-icon-sk title="Change sort order to ascending." @click=${() => this.changeSort(index)}></arrow-drop-down-icon-sk>`;
    }
    return html`<sort-icon-sk title="Sort this column." @click=${() => this.changeSort(index)}></sort-icon-sk>`;
  }

  private changeSort(column: number) {
    if (this.sortBy === column) {
      if (this.sortDirection === 'down') {
        this.sortDirection = 'up';
      } else {
        this.sortDirection = 'down';
      }
    }
    this.sortBy = column;
    this._render();
  }

  private tableRows(): TemplateResult[] {
    const traceset = this.df!.traceset;
    const sortedRowKeys = Object.keys(traceset).sort((a: string, b: string) => {
      let ret = 0;
      if (this.sortBy === -1) {
        if (a < b) {
          ret = -1;
        } else if (b < a) {
          ret = 1;
        } else {
          ret = 0;
        }
      } else {
        ret = traceset[a][this.sortBy] - traceset[b][this.sortBy];
      }

      if (this.sortDirection === 'down') {
        ret = -ret;
      }
      return ret;
    });
    const ret: TemplateResult[] = [];
    sortedRowKeys.forEach((key) => {
      ret.push(html`<tr><th class=key>${key}</th>${this.rowValues(key)}</tr>`);
    });
    return ret;
  }

  private rowValues(key: string): TemplateResult[] {
    return this.df!.traceset[key]!.map((value) => html`<td>${value.toPrecision(4)}</td>`);
  }
}

define('pivot-table-sk', PivotTableSk);

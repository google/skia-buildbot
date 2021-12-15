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

export class PivotTableSk extends ElementSk {
  private df: DataFrame | null = null;

  private req: pivot.Request | null = null;

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
    return html`<tr><th class=empty></th>${this.req!.summary!.map((summaryOperation) => html`<th>${summaryOperation}</th>`)}</tr>`;
  }

  private tableRows(): TemplateResult[] {
    const sortedRowKeys = Object.keys(this.df!.traceset).sort();
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

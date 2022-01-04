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
import { operationDescriptions, validateAsPivotTable } from '../pivotutil';

import 'elements-sk/icon/sort-icon-sk';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';
import { fromKey } from '../paramtools';

type direction = 'up' | 'down';

type sortKind = 'keyValues' | 'summaryValues'
export class PivotTableSk extends ElementSk {
  private df: DataFrame | null = null;

  private req: pivot.Request | null = null;

  /** Maps each traceKey to a list of the values for each key in the traceID,
   * where the order is determined by this.req.group_by.
   *
   * That is ',arch=arm,config=8888,' maps to ['8888', 'arm'] if
   * this.req.group_by is ['config', 'arch'].
   *  */
  private keyValues: {[key: string]: string[]} = {}

  /** The index to sort by, the value is interpreted differently based
   * on the value of this.sortKind:
   *
   * If this.sortKind === 'keyValues' then it is an index in the trace values.
   * If this.sortKind === 'summaryValues' then it is an index in keyValues.
   *  */
  private sortBy: number = 0;

  private sortKind: sortKind = 'summaryValues';

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
    this.keyValues = {};

    Object.keys(this.df.traceset).forEach((traceKey) => {
      // Parse the key.
      const ps = fromKey(traceKey);
      // Store the values for each key in group_by order.
      this.keyValues[traceKey] = this.req!.group_by!.map((colName) => ps[colName]);
    });
    this._render();
  }

  private tableHeader(): TemplateResult {
    return html`
    <tr>
      ${this.keyColumnHeaders()}
      ${this.summaryColumnHeaders()}
    </tr>`;
  }

  private keyColumnHeaders(): TemplateResult[] {
    return this.req!.group_by!.map((groupBy: string, index: number) => html`<th>${this.sortArrow(index, 'keyValues')} ${groupBy}</th>`);
  }

  private summaryColumnHeaders(): TemplateResult[] {
    return this.req!.summary!.map((summaryOperation, index) => html`<th>${this.sortArrow(index, 'summaryValues')} ${operationDescriptions[summaryOperation]}</th>`);
  }

  private sortArrow(index: number, kind: sortKind): TemplateResult {
    if (this.sortKind === kind) {
      if (index === this.sortBy) {
        if (this.sortDirection === 'up') {
          return html`<arrow-drop-up-icon-sk title="Change sort order to descending." @click=${() => this.changeSort(index, kind)}></arrow-drop-up-icon-sk>`;
        }
        return html`<arrow-drop-down-icon-sk title="Change sort order to ascending." @click=${() => this.changeSort(index, kind)}></arrow-drop-down-icon-sk>`;
      }
    }
    return html`<sort-icon-sk title="Sort this column." @click=${() => this.changeSort(index, kind)}></sort-icon-sk>`;
  }

  private changeSort(column: number, kind: sortKind) {
    if (this.sortBy === column) {
      if (this.sortDirection === 'down') {
        this.sortDirection = 'up';
      } else {
        this.sortDirection = 'down';
      }
    }
    this.sortKind = kind;
    this.sortBy = column;
    this._render();
  }

  private tableRows(): TemplateResult[] {
    const traceset = this.df!.traceset;
    const sortedRowKeys = Object.keys(traceset).sort((a: string, b: string) => {
      let ret = 0;
      if (this.sortKind === 'keyValues') {
        const aString = this.keyValues[a][this.sortBy];
        const bString = this.keyValues[b][this.sortBy];
        if (aString < bString) {
          ret = -1;
        } else if (bString < aString) {
          ret = 1;
        } else {
          // Fall back to sorting by the full keys if aString === bString.
          if (a < b) {
            return -1;
          } if (b < a) {
            return 1;
          }
          return 0;
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
      ret.push(html`<tr>${this.keyRowValues(key)}${this.summaryRowValues(key)}</tr>`);
    });
    return ret;
  }

  private keyRowValues(traceKey: string): TemplateResult[] {
    return this.keyValues[traceKey].map((value) => html`<th class=key>${value}</th>`);
  }

  private summaryRowValues(key: string): TemplateResult[] {
    return this.df!.traceset[key]!.map((value) => html`<td>${PivotTableSk.displayValue(value)}</td>`);
  }

  /** Converts vec32.MissingDataSentinel values into '-'. */
  private static displayValue(value: number): string {
    // TODO(jcgregorio) Have a common definition of vec32.MissingDataSentinel in
    // TS and Go code.
    if (value === 1e32) {
      return '-';
    }
    return value.toPrecision(4);
  }
}

define('pivot-table-sk', PivotTableSk);

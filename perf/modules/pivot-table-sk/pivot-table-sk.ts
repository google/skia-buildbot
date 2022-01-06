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
import { toParamSet } from 'common-sk/modules/query';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { pivot, DataFrame, TraceSet } from '../json';
import { operationDescriptions, validateAsPivotTable } from '../pivotutil';

import '../../../infra-sk/modules/paramset-sk';
import 'elements-sk/icon/sort-icon-sk';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';
import { fromKey } from '../paramtools';

/** The direction a column is sorted in. */
export type direction = 'up' | 'down';

/** The different kinds of columns. */
export type columnKind = 'keyValues' | 'summaryValues';

/** Type for a function that can be passed to Array.sort(). */
export type compareFunc = (a: string, b: string)=> number;

/** For each key in a traceset, this stores the values for key,value pair in the
 * traceid that appear in pivot.Request.group_by, and holds them in the order as
 * determined by pivot.Request.group_by.
 */
export type KeyValues = {[key: string]: string[]};

/** Represents a how a single column in the table is to be sorted.
 */
export class SortSelection {
  // The column to sort on, the value is interpreted differently based
  // on the value of this.sortKind:
  //
  // If this.kind === 'keyValues' then it is an index into the keyValues.
  // If this.Kind === 'summaryValues' then it is an index into the trace values.
  column: number = 0;

  kind: columnKind = 'summaryValues';

  dir: direction = 'up';

  constructor(column: number, kind: columnKind, dir: direction) {
    this.column = column;
    this.kind = kind;
    this.dir = dir;
  }

  toggleDirection(): void {
    if (this.dir === 'down') {
      this.dir = 'up';
    } else {
      this.dir = 'down';
    }
  }

  /** Returns a compareFunc that sorts based on the state of this SortSelection.
   */
  buildCompare(traceset: TraceSet, keyValues: {[key: string]: string[]}): compareFunc {
    const compare = (a: string, b: string): number => {
      let ret = 0;
      if (this.kind === 'keyValues') {
        const aString = keyValues[a][this.column];
        const bString = keyValues[b][this.column];
        if (aString < bString) {
          ret = -1;
        } else if (bString < aString) {
          ret = 1;
        } else {
          return 0;
        }
      } else {
        ret = traceset[a][this.column] - traceset[b][this.column];
      }

      if (this.dir === 'down') {
        ret = -ret;
      }
      return ret;
    };

    return compare;
  }
}

/**
 * Keeps one SortSelection for each column being displayed. As the user clicks
 * on columns the function `selectColumnToSortOn` can be called to keep
 * `this.history` up to date.
 *
 * This enables better sorting behavior, i.e. when you click on col A to sort,
 * then on col B to sort, if there are ties in col B they are broken by the
 * existing order in col A, just like you would get when sorting by columns in a
 * spreadsheet.
 *
 * This is not technically 'stable sort', while each sort action by the user
 * looks like it is doing a stable sort, which is the goal, we are really doing
 * an absolute sort based on a memory of all previous sort actions.
 */
export class SortHistory {
  /** Columns will be sorted by the first entry in history. If that yields a
   * tie, then the second entry in history will be used to break the tie, etc.
   */
  history: SortSelection[] = []

  constructor(numGroupBy: number, numSummaryValues: number) {
    for (let i = 0; i < numSummaryValues; i++) {
      this.history.push(new SortSelection(i, 'summaryValues', 'up'));
    }
    for (let i = 0; i < numGroupBy; i++) {
      this.history.push(new SortSelection(i, 'keyValues', 'up'));
    }
  }

  /** Moves the selected column to the front of the list for sorting, and also
   * reverses its current direction.
   */
  selectColumnToSortOn(column: number, kind: columnKind): void {
    // Remove the matching SortSelection from history.
    let removed: SortSelection[] = [];
    for (let i = 0; i < this.history.length; i++) {
      if (column === this.history[i].column && kind === this.history[i].kind) {
        removed = this.history.splice(i, 1);
        break;
      }
    }

    // Toggle its direction.
    removed[0].toggleDirection();

    // Then add back to the beginning of the list.
    this.history.unshift(removed[0]);
  }

  /** Returns a compareFunc that sorts based on the state of all the
   *  SortSelections in history.
   */
  buildCompare(traceset: TraceSet, keyValues: {[key: string]: string[]}): compareFunc {
    const compares = this.history.map((sel: SortSelection) => sel.buildCompare(traceset, keyValues));
    const compare = (a: string, b: string): number => {
      let ret = 0;
      // Call each compareFunc in `compares` until one of them produces a
      // non-zero result. If all calls return 0 then this compare function also
      // returns 0.
      compares.some((colCompare: compareFunc) => {
        ret = colCompare(a, b);
        return ret;
      });
      return ret;
    };
    return compare;
  }
}

export function keyValuesFromTraceSet(traceset: TraceSet, req: pivot.Request): KeyValues {
  const ret: KeyValues = {};
  Object.keys(traceset).forEach((traceKey) => {
    // Parse the key.
    const ps = fromKey(traceKey);
    // Store the values for each key in group_by order.
    ret[traceKey] = req.group_by!.map((colName) => ps[colName]);
  });
  return ret;
}

export class PivotTableSk extends ElementSk {
  private df: DataFrame | null = null;

  private req: pivot.Request | null = null;

  private query: string = ''

  /** Maps each traceKey to a list of the values for each key in the traceID,
   * where the order is determined by this.req.group_by.
   *
   * That is ',arch=arm,config=8888,' maps to ['8888', 'arm'] if
   * this.req.group_by is ['config', 'arch'].
   *  */
  private keyValues: KeyValues = {}

  private sortHistory: SortHistory | null = null;

  // The comparison function to use to sort the table.
  private compare: compareFunc | null = null;

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
    return html`
    ${ele.queryDefinition()}
    <table>
      ${ele.tableHeader()}
      ${ele.tableRows()}
    </table>`;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  set(df: DataFrame, req: pivot.Request, query: string): void {
    this.df = df;
    this.req = req;
    this.query = query;
    this.keyValues = keyValuesFromTraceSet(this.df.traceset, this.req);
    this.sortHistory = new SortHistory(req.group_by!.length, req.summary!.length);
    this.compare = this.sortHistory.buildCompare(this.df.traceset, this.keyValues);
    this._render();
  }

  private queryDefinition(): TemplateResult {
    return html`
    <div class=querydef>
      <div>
        <span class=title>Query</span>
        <paramset-sk .paramsets=${[toParamSet(this.query)]}></paramset-sk>
      </div>
      <div>
        <span class=title>Group by:</span>
        ${this.req!.group_by!.join(', ')}
      </div>
      <div>
        <span class=title>Operation:</span>
        ${operationDescriptions[this.req!.operation]}
      </div>
      <div>
        <span class=title>Summaries:</span>
        ${this.req!.summary!.map((op: pivot.Operation) => operationDescriptions[op]).join(', ')}
      </div>
    </div>`;
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

  private sortArrow(column: number, kind: columnKind): TemplateResult {
    const firstSortSelection = this.sortHistory!.history[0];
    if (firstSortSelection.kind === kind) {
      if (column === firstSortSelection.column) {
        if (firstSortSelection.dir === 'up') {
          return html`<arrow-drop-up-icon-sk title="Change sort order to descending." @click=${() => this.changeSort(column, kind)}></arrow-drop-up-icon-sk>`;
        }
        return html`<arrow-drop-down-icon-sk title="Change sort order to ascending." @click=${() => this.changeSort(column, kind)}></arrow-drop-down-icon-sk>`;
      }
    }
    return html`<sort-icon-sk title="Sort this column." @click=${() => this.changeSort(column, kind)}></sort-icon-sk>`;
  }

  private changeSort(column: number, kind: columnKind) {
    this.sortHistory!.selectColumnToSortOn(column, kind);
    this.compare = this.sortHistory!.buildCompare(this.df!.traceset, this.keyValues);
    this._render();
  }

  private tableRows(): TemplateResult[] {
    const traceset = this.df!.traceset;
    const sortedRowKeys = Object.keys(traceset).sort(this.compare!);
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

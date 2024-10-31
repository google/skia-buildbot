/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/checkbox-sk';
import { AnomaliesTableColumn, AnomaliesTableResponse, AnomaliesTableRow } from '.';

export class AnomaliesTableSk extends ElementSk {
  private _table: HTMLDialogElement | null = null;

  private data: AnomaliesTableResponse;

  private checked: boolean | null = false;

  constructor() {
    super(AnomaliesTableSk.template);

    console.log('anomaliestable');
    this.data = AnomaliesTableSk.dummyData();
  }

  private static dummyData(): AnomaliesTableResponse {
    const column: AnomaliesTableColumn = {
      check_header: false,
      graph_header: 'graph',
      bug_id: '12345',
      end_revision: 'endrevise',
      master: 'master',
      bot: 'bot',
      test_suite: 'test_suite',
      test: 'test',
      change_direction: 'changedirection',
      percent_changed: 'percentChanged',
      absolute_delta: 'absoluteDelta',
    };

    const row: AnomaliesTableRow = {
      columns: [column],
    };

    return {
      table: [row],
    };
  }

  private static template = (ele: AnomaliesTableSk) =>
    html`<table id="anomalies">
      <thead>
        <tr class="headers">
          <th id="check_header">
            <checkbox-sk id="header-checkbox" ?checked=${ele.checked}>/checkbox-sk>
          </th>
          <th
            id="graph_header"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Graph
          </th>
          <th
            id="bug_id"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Bug ID
          </th>
          <th
            id="end_revision"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Revisions
          </th>
          <th
            id="master"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Master
          </th>
          <th
            id="bot"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Bot
          </th>
          <th
            id="testsuite"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Test Suite
          </th>
          <th
            id="test"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Test
          </th>
          <th id="change_direction">Change Direction</th>
          <th
            id="percent_changed"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Delta %
          </th>
          <th
            id="absolute_delta"
            @click=${() => {
              ele.columnHeaderClicked();
            }}>
            Abs Delta
          </th>
        </tr>
      </thead>
      <tbody>
        ${AnomaliesTableSk.rows(ele)}
      </tbody>
    </table> `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._table = this.querySelector('#anomalies');
  }

  private static rows = (ele: AnomaliesTableSk) =>
    ele.data!.table!.map(
      (row) => html`
        <tr>
          ${AnomaliesTableSk.columns(row!)}
        </tr>
      `
    );

  private static columns = (row: AnomaliesTableRow) =>
    row.columns!.map((col) => {
      const ret = [];
      ret.push(html`
        <td>${col!.check_header}</td>
        <td>${col!.graph_header}</td>
        <td>${col!.bug_id}</td>
        <td>${col!.end_revision}</td>
        <td>${col!.master}</td>
        <td>${col!.bot}</td>
        <td>${col!.test_suite}</td>
        <td>${col!.test}</td>
        <td>${col!.change_direction}</td>
        <td>${col!.percent_changed}</td>
        <td>${col!.absolute_delta}</td>
      `);
      return ret;
    });

  /**
   * Callback for the click event for a column header.
   * @param {Event} event Clicked event.
   * @param {Object} detail Detail Object.
   */
  columnHeaderClicked(): void {
    this.sort();
  }

  // TODO(jiaxindong)
  // b/375640853 Group anomalies and sort with the revision range in either high or low direction
  /**
   * Sorts the alert list according to the current values of the properties
   * sortDirection and sortBy.
   */
  private sort() {}
}

define('anomalies-table-sk', AnomaliesTableSk);

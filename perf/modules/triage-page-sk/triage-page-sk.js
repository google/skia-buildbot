/**
 * @module module/triage-page-sk
 * @description <h2><code>triage-page-sk</code></h2>
 *
 * Allows triaging clusters.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

const _allFilters = (ele) => ele._all_filter_options.map(
  (o) => html`
    <option
      ?selected=${ele._state.filter === o.value}
      value=${o.value}
      title=${o.title}
      >${o.display}</option>`);


const _statusItems = (ele) => ele._currentState.map((item) => html`
  <table>
    <tr>
      <th>Alert</th>
      <td><a href='/a/?${item.alert.id}'>${item.alert.display_name}</a></td>
    </tr>
    <tr>
      <th>Commit</th>
      <td><commit-detail-sk .cid=${item.commit}></commit-detail-sk></td>
    </tr>
    <tr>
      <th>Step</th>
      <td>${item.step}/${item.total}</td>
    </tr>
  </table>
`);

const _headers = (ele) => ele._reg.header.map((item) => {
  let displayName = item.display_name;
  if (!item.display_name) {
    displayName = item.query.slice(0, 10);
  }
  // The colspan=2 is important since we will have two columns under each
  // header, one for high and one for low.
  return html`<th colspan=2><a href='/a/?${item.id}'>${displayName}</a></th>`;
});

const _subHeaders = (ele) => ele._reg.header.map((item, index) => {
  const ret = [];
  if (ele._stepDownAt(index)) {
    ret.push('<th>Low</th>');
  }
  if (ele._stepUpAt(index)) {
    ret.push('<th>High</th>');
  }
  // If we have only one of High or Low we stuff in an empty th to match
  // colspan=2 above.
  if (ele._notBoth(index)) {
    ret.push('<th></th>');
  }
});

function _full_summary(frame, summary) {
  return {
    frame: frame,
    summary: summary,
  }
}

const _lowCell = (ele, rowIndex, col, colIndex) => {
  if (col.low) {
    return html`<triage-status-sk .alert=${ele._alertAt(colIndex)} .cluster_type=low .full_summary=${_full_summary(item.frame, item.low)} .triage=${col.low_status}></triage-status-sk> `;
  } else {
    return html`<a class=dot title='No clusters found.' href='/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(colindex)}'>∅</a> `;
  }
}

const _highCell = (ele, rowIndex, col, colIndex) => {
  if (col.high) {
    return html`<triage-status-sk .alert=${ele._alertAt(colIndex)} .cluster_type=high .full_summary=${_full_summary(item.frame, item.high)} .triage=${col.high_status}></triage-status-sk> `;
  } else {
    return html`<a class=dot title='No clusters found.' href='/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(colindex)}'>∅</a> `;
  }
}

const _columns = (ele, row, rowIndex) => row.columns.map((col, colIndex) => {
  ret = [];

  if (ele._stepDownAt(index)) {
    ret.push(html`
      <td class=cluster>
        ${_lowCell(ele, rowIndex, col, colIndex )}
      </td>
    `);
  }
  if (ele._stepDownAt(index)) {
    ret.push(html`
      <td class=cluster>
        ${_highCell(ele, rowIndex, col, colIndex )}
      </td>
    `);
  }

  if (ele._notBoth(index)) {
    ret.push('<td></td>');
  }
  return ret;
});

html`
  <template is="dom-if" if="[[_stepDownAt(index)]]">
  </template>

  <template is="dom-if" if="[[_stepUpAt(index)]]">
    <td class=cluster>
      <template is="dom-if" if="[[item.high]]">
        <triage-status-sk alert="[[_alertAt(index)]]" cluster_type=high full_summary="[[_full_summary(item.frame, item.high)]]" triage="[[item.high_status]]"></triage-status-sk>
      </template>
      <template is="dom-if" if="[[_not(item.high)]]">
        <a class=dot title="No clusters found." href="/g/c/[[_hashFrom(tableIndex)]]?query=[[_encQueryFrom(index)]]">[[_display(tableIndex,state.end)]]</a>
      </template>
    </td>
  </template>

  <template is="dom-if" if="[[_notBoth(index)]]">
    <td></td><!-- Dummy column for colspan. -->
  </template>

  `;

const _rows= (ele) => ele._reg.table.map((row, rowIndex) => html`
  <tr>
    <td class=fixed>
      <commit-detail-sk .cid=${row.cid}></commit-detail-sk>
    </td>
    ${_columns(ele, row, rowIndex)}
  </tr>`);

const template = (ele) => html`
  <header>
    <details>
      <summary>
        <h2 id=filter>Filter</h2>
      </summary>
      <h3>Which commits to display.</h3>
      <select
        @input=${ele._updateRange}
        >
        <option ?selected=${ele._state.subset==='all'}       value=all title='Show results for all commits in the time range.'>All</option>
        <option ?selected=${ele._state.subset==='flagged'}   value=flagged title='Show only the commits with regressions in the given time range regardless of triage status.'>Regressions</option>
        <option ?selected=${ele._state.subset==='untriaged'} value=untriaged title='Show only commits with untriaged regressions in the given time range.'>Untriaged</option>
      </select>

      <h3>Which alerts to display.</h3>

      <select @input=${ele._updateRange}>
        ${_allFilters(ele)}
      </select>
    </details>
    <details>
      <summary>
        <h2 id="range">Range</h2>
      </summary>
      <day-range-sk id=range @day-range-change=${ele._rangeChange}></day-range-sk>
    </details>
    <details ?open=${ele._statusOpen} @toggle=${ele._statusToggle}>
      <summary>
        <h2>Status</h2>
      </summary>
      <div>
        <p>The current work on detecting regressions:</p>
        <div class=status>
          ${_statusItems(ele)}
        </div>
      </div>
    </details>
  </header>
  <spinner-sk id=spinner></spinner-sk>

  <dialog>
  <cluster-summary2-sk
    @open-keys=${ele._openKeys}
    @triaged=${ele._triaged}
    full_summary=${_dialog_state.full_summary}
    triage=${ele._dialog_state.triage}>
  </cluster-summary2-sk>
  <div class=buttons>
    <button @click=${ele._close}>Close</button>
  </div>
  </dialog>

  <table @start-triage=${ele._triage_start}>
    <tr>
      <th>Commit</th>
      ${_headers(ele)}
    </tr>
    <tr>
      <th></th>
      ${_subHeaders(ele)}
    </tr>
    ${_rows(ele)}
  </table>
  </template>
`;

window.customElements.define('triage-page-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }

  _stepUpAt(index) {
    const dir = this._reg.header[index].direction;
    return  dir == 'UP' || dir == 'BOTH';
  }

  _stepDownAt(index) {
    const dir = this._reg.header[index].direction;
    return  dir == 'DOWN' || dir == 'BOTH';
  }

  _notBoth(index) {
    return this._reg.header[index].direction != 'BOTH';
  }

  _alertAt(index) {
    return this._reg.header[index];
  }

  _encQueryFrom(colIndex) {
    return encodeURIComponent(this._reg.header[colIndex].query);
  }

  _hashFrom(rowIndex) {
    return this._reg.table[rowIndex].cid.hash;
  }

  /** @prop _all_filter_options {string} Used to populate the filter select
   * control.*/
  get _all_filter_options() {
    const ret = [
      {
        value: 'ALL',
        title: 'Show all alerts.',
        display: 'Show all alerts.',
      },
      {
        value: 'OWNER',
        title: 'Show only the alerts owned by the logged in user (or all alerts if the user doesn\'t own any alerts).',
        display: 'Show alerts you own.',
      },
    ];
    this._reg.categories.forEach((cat) => {
      const displayName = cat || '(default)';
      ret.push({
        value: `cat:${cat}`,
        title: 'Show only the alerts in the ${displayName} category.',
        display: 'Category: ${displayName}',
      });
    });
    return ret;
  }


});

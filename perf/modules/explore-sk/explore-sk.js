/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

import 'elements-sk/spinner-sk'
import 'elements-sk/tabs-sk'
import 'elements-sk/checkbox-sk'
import 'elements-sk/tabs-panel-sk'
import 'elements-sk/styles/buttons'

import '../plot-simple-sk'
import '../domain-picker-sk'
import '../query-sk'
import '../query-summary-sk'
import '../paramset-sk'
import '../commit-detail-panel-sk'
import '../json-source-sk'

const template = (ele) => html`
    <plot-simple-sk width=1024 height=256 id=plot></plot-simple-sk>

    <div id=buttons>
      <domain-picker-sk id=range state=${ele.state} @domain-changed=${ele._rangeChange}></domain-picker-sk>
      <button @click=${ele._removeHighlighted} title="Remove all the highlighted traces.">Remove Highlighted</button>
      <button @click=${ele._removeAll} title="Remove all the traces.">Remove All</button>
      <button @click=${ele._highlightedOnly} title="Remove all but the highlighted traces.">Highlighted Only</button>
      <button @click=${ele._clearHighlights} title="Remove highlights from all traces.">Clear Highlights</button>
      <button @click=${ele._resetAxes} title="Reset back to the original zoom level.">Reset Axes</button>
      <button @click=${ele._shiftLeft} title="Move 10 commits in the past.">&lt;&lt; ${ele._numShift}</button>
      <button @click=${ele._shiftBoth} title="Expand the display ${ele._numShift} commits in both directions.">&lt;&lt; ${+ele._numShift} &gt;&gt;</button>
      <button @click=${ele._shiftRight} title="Move 10 commits in the future.">${+ele._numShift} &gt;&gt;</button>
      <button @click=${ele._zoomToRange} id=zoom_range disabled title="Fit the time range to the current zoom window.">Zoom Range</button>
      <span title="Number of commits skipped between each point displayed." ?hidden=${_isZero(ele._dataframe.skip)} id=skip>${ele._dataframe.skip}</span>
      <checkbox-sk name=zero @click=${ele._zeroChanged} ?checked=${ele.state.show_zero} title="Toggle the presence of the zero line.">Zero</paper-checkbox>
      <checkbox-sk name=auto @click=${ele._autoRefreshChanged} ?checked=${ele.state.auto_refresh} title="Auto-refresh the data displayed in the graph.">Auto-Refresh</paper-checkbox>
      <button @click=${ele._normalize} title="Apply norm() to all the traces.">Normalize</button>
      <button @click=${ele._scale_by_ave} title="Apply scale_by_ave() to all the traces.">Scale By Ave</button>
      <button @click=${ele._csv} title="Download all displayed data as a CSV file.">CSV</button>
      <a href="" target=_blank download="traces.csv" id=csv_download></a>
      <div id=spin-container>
        <spinner-sk id=spinner></spinner-sk>
        <span id=percent></span>
      </div>
    </div>

    <div id=tabs>
      <tabs-sk id=detailtabl>
        <button>Query</button>
        <button>Params</button>
        <button id=commitsTab disabled>Details</button>
      </tabs-sk>
        <tabs-panel-sk id=queryTab>
          <div class="layout horizontal">
            <query-sk id=query></query-sk>
            <div class="layout vertical" id=selections>
              <h3>Selections</h3>
              <query-summary-sk id=summary></query-summary-sk>
              <div>
                Matches: <span id=matches></span>
              </div>
              <button on-tap="_add" class=action>Plot</button>
            </div>
          </div>
          <h3>Calculated Traces</h3>
          <div class="layout horizontal center">
            <paper-input-decorator floatingLabel label="Formula" flex>
              <textarea id="formula" rows=3 cols=80></textarea>
            </paper-input-decorator>
            <button on-tap="_addCalculated" class=action>Add</button>
            <a href="/help/" target=_blank><iron-icon icon="help"></iron-icon></a>
          </div>
        </tabs-panel-sk>
        <tabs-panel-sk>
          <paper-input floatingLabel label="Trace ID" readonly id=trace_id></paper-input>
          <paramset-sk id=paramset class=hidden clickable_values></paramset-sk>
        </div>
        <div id=details class="layout horizontal wrap hidden">
          <paramset-sk id=simple_paramset clickable_values></paramset-sk>
          <div class="layout vertical">
            <commit-detail-panel-sk id=commits></commit-detail-panel-sk>
            <json-source-sk id=jsonsource></json-source-sk>
          </div>
        </div>
    </div>
  `;

window.customElements.define('explore-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }

});

/**
 * @module module/query-chooser-sk
 * @description <h2><code>query-chooser-sk</code></h2>
 *
 * Displays the current value for a selection along with an edit button
 * that pops up a query-sk dialog to change the selection.
 *
 * Emits the same events as query-sk.
 *
 * @attr {string} current_query - The current query formatted as a URL formatted query string.
 *
 * @attr {string} count_url - The  URL to POST the query to, passed down to quuery-count-sk.
 *
 */
import { LitElement, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { fromParamSet, ParamSet, toParamSet } from '../../../infra-sk/modules/query';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { ParamSetSkRemoveClickEventDetail } from '../../../infra-sk/modules/paramset-sk/paramset-sk';

import '../../../infra-sk/modules/paramset-sk';
import '../../../infra-sk/modules/query-sk';

import '../query-count-sk';

@customElement('query-chooser-sk')
export class QueryChooserSk extends LitElement {
  @property({ type: String, reflect: true })
  current_query = '';

  @property({ type: Object })
  paramset: ParamSet = {};

  @property({ type: Array })
  key_order: string[] = [];

  @property({ type: String })
  count_url = '';

  @state()
  private _showDialog = false;

  protected createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <div class="row">
        <button @click=${this._editClick}>Edit</button>
        <paramset-sk
          id="summary"
          removable_values
          @paramset-value-remove-click=${this.paramsetRemoveClick}
          .paramsets=${[toParamSet(this.current_query)]}></paramset-sk>
      </div>
      <div id="dialog" class="${this._showDialog ? 'display' : ''}">
        <query-sk
          current_query=${this.current_query}
          .paramset=${this.paramset}
          .key_order=${this.key_order}
          @query-change=${this._queryChange}></query-sk>
        <div class="matches">
          Matches:
          <query-count-sk
            url=${this.count_url}
            current_query=${this.current_query}></query-count-sk>
        </div>
        <button @click=${this._closeClick}>Close</button>
      </div>
    `;
  }

  private _editClick() {
    this._showDialog = true;
  }

  private _closeClick() {
    this._showDialog = false;
  }

  private _queryChange(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.current_query = e.detail.q;
    this.dispatchEvent(new CustomEvent('query-change', { detail: e.detail, bubbles: true }));
  }

  private paramsetRemoveClick(e: CustomEvent<ParamSetSkRemoveClickEventDetail>) {
    const paramset = toParamSet(this.current_query);
    const values = paramset[e.detail.key] || [];
    const index = values.indexOf(e.detail.value);
    if (index > -1) {
      values.splice(index, 1);
    }
    this.current_query = fromParamSet(paramset);
  }
}

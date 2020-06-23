/**
 * @module modules/search-controls-sk
 * @description <h2><code>search-controls-sk</code></h2>
 *
 * This is a general element to be used by all pages that
 * call a search endpoint on the backend.
 * It encapsulates the state of the query. When that state
 * is changed through some of the controls it updates the URL
 * and send an update to the host element to reload data based
 * on the new query state.
 *
 * The state object contains these fields:
 *   - pos: show positive (boolean).
 *   - neg: show negative (boolean).
 *   - unt: show untriaged (boolean).
 *   - include: show ignored digests (boolean).
 *   - head: only digests that are currently in HEAD.
 *   - query: query string to select configuration.
 *
 * Attributes:
 *   state - The current query state.
 *
 *   disabled - Boolean to indicate whether to disable all the controls.
 *   beta - Boolean to enable beta-level functions.
 *
 * Events:
 *   'state-change' - Fired when the state of the query changes and
 *       it needs to be reloaded. The 'detail' field of the event contains
 *       the new state represented by the controls.
 *
 * Methods:
 *   setState(state) - Set the state of the controls to 'state'.
 *
 *   setParamSet(params) - Sets the parameters of the enclosed query-dialog-sk element
 *       and enables the controls accordingly.
 *
 *   setCommitInfo(commitinfo) - Where commitinfo is an array of objects of the form:
 *
 *     {
 *       author: "foo@example.org"
 *       commit_time: 1428574804
 *       hash: "d9f8862ab6bed4195cbfe5dda48693e1062b01e2"
 *     }
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// TODO(lovisolo): Implement.

export class SearchControlsSk extends ElementSk {
  private static _template = (el: SearchControlsSk) => html`
    Hello, world!
  `;

  constructor() {
    super(SearchControlsSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('search-controls-sk', SearchControlsSk);

/**
 * @module modules/edit-ignore-rule-sk
 * @description <h2><code>edit-ignore-rule-sk</code></h2>
 *
 * The edit-ignore-rule-sk element shows the pieces of a Gold ignore rule
 * and allows for the modification of them.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { diffDate } from '../../../common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { html } from 'lit-html';
import { humanReadableQuery } from '../common';

import '../../../infra-sk/modules/query-sk';

const template = (ele) => html`
  <div class=columns>
    <label>Expires in</label>
    <input placeholder="(e.g. 2w, 4h)" id=expires value=${ele._expiresText}>
  </div>

  <div class=columns>
    <textarea placeholder="Add a note, skia:1234" id=note>${ele._note}</textarea>
    <div class=query>${humanReadableQuery(ele.query)}</div>
  </div>

  <query-sk .paramset=${ele.params} .current_query=${ele.query} hide_invert hide_regex
    @query-change=${ele._queryChanged}></query-sk>

   <div class=error ?hidden=${!ele._errMsg}>${ele._errMsg}</div>
`;


define('edit-ignore-rule-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._params = {};
    this._query = '';
    this._note = '';
    this._expiresText = '';
    this._errMsg = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop params {Object} A map of String -> Array<String> which maps keys to potential values.
   *   objects.
   */
  get params() { return this._params; }
  set params(p) {
    this._params = p;
    this._render();
  }

  get query() { return this._query; }
  set query(q) {
    this._query = q;
    this._render();
  }

  get expires() {
    // Note, this is a string like 2w, 3h - it will be parsed server-side.
    return $$('#expires').value;
  }
  set expires(d) {
    // We are given a date string, we turn it into the human readable text. There might be some
    // loss of fidelity (e.g. a time of 2 weeks minus 10 minutes gets rounded to 2w gets rounded),
    // but that's ok - the developers don't expect expiration times to be ultra precise.
    const nowMS = Date.now();
    const newMS = Date.parse(d);
    if (!newMS || newMS < nowMS) {
      this._expiresText = '';
    } else {
      this._expiresText = diffDate(d);
    }
    this._render();
  }

  get note() { return $$('#note').value; }
  set note(n) {
    this._note = n;
    this._render();
  }

  _queryChanged(e) {
    e.preventDefault();
    e.stopPropagation();
    this.query = e.detail.q;
  }

  /**
   * Resets input, outputs, and error, other than the params. The params field will likely
   * not change over the lifetime of this element.
   */
  reset() {
    this._query = '';
    this._note = '';
    this._expiresText = '';
    this._errMsg = '';
    this._render();
  }

  /**
   * Checks that all the required inputs (query and expires) have non-empty values. If so, it
   * returns true, otherwise, it will display an error on the element.
   * @return {boolean}
   */
  verifyFields() {
    if (this.query && this.expires) {
      this._errMsg = '';
      this._render();
      return true;
    }
    this._errMsg = 'Must specify a non-empty filter and an expiration';
    this._render();
    return false;
  }
});

/**
 * @module modules/edit-ignore-rule-sk
 * @description <h2><code>edit-ignore-rule-sk</code></h2>
 *
 * The edit-ignore-rule-sk element shows the pieces of a Gold ignore rule and allows for the
 * modification of them.
 *
 * TODO(kjlubick) Add client-side validation of expires values.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { humanReadableQuery } from '../common';

import '../../../infra-sk/modules/query-sk';

const template = (ele) => html`
  <div class=columns>
    <label for=expires>Expires in</label>
    <input placeholder="(e.g. 2w, 4h)" id=expires value=${ele._expiresText}>
  </div>

  <div class=columns>
    <textarea placeholder="Enter a note, e.g 'skia:1234'" id=note>${ele._note}</textarea>
    <div class=query>${humanReadableQuery(ele.query)}</div>
  </div>

  <query-sk .paramset=${ele.paramset} .current_query=${ele.query} hide_invert hide_regex
    @query-change=${ele._queryChanged}></query-sk>

   <div class=error ?hidden=${!ele._errMsg}>${ele._errMsg}</div>
`;


define('edit-ignore-rule-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._paramset = {};
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
   * @prop paramset {Object} A map of String -> Array<String> containing the available key/value
   *       pairs from which ignore rules may be built. For example, {'os': ['linux', 'windows']}.
   */
  get paramset() { return this._paramset; }

  set paramset(p) {
    this._paramset = p;
    this._render();
  }

  /**
   * @prop query {String} A URL-encoded string containing the selected query. For example,
   *       'alpha=beta&mind%20the_gap=space'.
   */
  get query() { return this._query; }

  set query(q) {
    this._query = q;
    this._render();
  }

  /**
   * @prop expires {String} The human readable shorthand for a time duration (e.g. '2w'). If set
   *       with a date, it will be converted into shorthand notation when displayed to the user.
   *       This time duration represents how long until the ignore rule "expires", i.e. when it
   *       should be re-evaluated if it is needed still.
   */
  get expires() {
    // Note, this is a string like 2w, 3h - it will be parsed server-side.
    return $$('#expires', this).value;
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

  /**
   * @prop note {String} A note, usually a comment, to accompany the ignore rule.
   */
  get note() { return $$('#note', this).value; }

  set note(n) {
    this._note = n;
    this._render();
  }

  _queryChanged(e) {
    // Stop the query-sk event from leaving this element to avoid confusing a parent element
    // with unexpected events.
    e.stopPropagation();
    this.query = e.detail.q;
  }

  /**
   * Resets input, outputs, and error, other than the paramset. The paramset field will likely
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
    this._errMsg = 'Must specify a non-empty filter and an expiration.';
    this._render();
    return false;
  }
});

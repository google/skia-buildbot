/**
 * @module modules/edit-ignore-rule-sk
 * @description <h2><code>edit-ignore-rule-sk</code></h2>
 *
 * The edit-ignore-rule-sk element shows the pieces of a Gold ignore rule and allows for the
 * modification of them.
 *
 * TODO(kjlubick) Add client-side validation of expires values.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { humanReadableQuery } from '../common';
import { ParamSet } from '../rpc_types';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';

import '../../../infra-sk/modules/query-sk';

export class EditIgnoreRuleSk extends ElementSk {

  private static template = (ele: EditIgnoreRuleSk) => html`
    <div class=columns>
      <label for=expires>Expires in</label>
      <input placeholder="(e.g. 2w, 4h)" id=expires value=${ele._expires}>
    </div>

    <div class=columns>
      <textarea placeholder="Enter a note, e.g 'skia:1234'" id=note>${ele._note}</textarea>
      <div class=query>${humanReadableQuery(ele.query)}</div>
    </div>

    <query-sk .paramset=${ele.paramset} .current_query=${ele.query} hide_invert hide_regex
      @query-change=${ele.queryChanged}></query-sk>

    <div>
      <input class=custom_key placeholder="specify a key">
      <input class=custom_value placeholder="specify a value">
      <button class=add_custom @click=${ele.addCustomParam}
        title="Add a custom key/value pair to ignore. For example, if adding a new test/corpus and
          you want to avoid spurious comments about untriaged digests, use this to add a rule
          before the CL lands.">
        Add Custom Param
       </button>
    </div>

    <div class=error ?hidden=${!ele.errMsg}>${ele.errMsg}</div>
  `;

  private _paramset: ParamSet = {};
  private _query = '';
  private _note = '';
  private _expires = '';
  private errMsg = '';

  constructor() {
    super(EditIgnoreRuleSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * Key/value pairs from which ignore rules may be built. For example,
   * `{'os': ['linux', 'windows']}`.
   */
  get paramset(): ParamSet { return this._paramset; }

  set paramset(p: ParamSet) {
    this._paramset = p;
    this._render();
  }

  /**
   * A URL-encoded string containing the selected query. For example,
   * 'alpha=beta&mind%20the_gap=space'`.
   */
  get query(): string { return this._query; }

  set query(q: string) {
    this._query = q;
    this._render();
  }

  /**
   * The human readable shorthand for a time duration (e.g. '2w'). If set with a date, it will be
   * converted into shorthand notation when displayed to the user. This time duration represents
   * how long until the ignore rule "expires", i.e. when it should be re-evaluated if it is needed
   * still.
   */
  get expires(): string {
    // Note, this is a string like 2w, 3h - it will be parsed server-side.
    return this.querySelector<HTMLInputElement>('#expires')!.value;
  }

  set expires(d: string) {
    // We are given a date string, we turn it into the human readable text. There might be some
    // loss of fidelity (e.g. a time of 2 weeks minus 10 minutes gets rounded to 2w gets rounded),
    // but that's ok - the developers don't expect expiration times to be ultra precise.
    const nowMS = Date.now();
    const newMS = Date.parse(d);
    if (!newMS || newMS < nowMS) {
      this._expires = '';
    } else {
      this._expires = diffDate(d);
    }
    this._render();
  }

  /** A note, usually a comment, to accompany the ignore rule. */
  get note(): string { return this.querySelector<HTMLInputElement>('#note')!.value; }

  set note(n: string) {
    this._note = n;
    this._render();
  }

  private addCustomParam() {
    const keyInput = this.querySelector<HTMLInputElement>('input.custom_key')!;
    const valueInput = this.querySelector<HTMLInputElement>('input.custom_value')!;

    const key = keyInput.value;
    const value = valueInput.value;
    if (!key || !value) {
      this.errMsg = 'Must specify both a key and a value.';
      this._render();
      return;
    }
    // Push the key/value to the _paramset so the query-sk can treat it like a normal value.
    const values = this._paramset[key] || [];
    values.push(value);
    this._paramset[key] = values;
    this.errMsg = '';
    // Add the selection to the query so it shows up for the user.
    const newParam = `${key}=${value}`;
    if (this._query) {
      this._query += `&${newParam}`;
    } else {
      this._query = newParam;
    }
    this._render();
  }

  private queryChanged(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
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
    this._expires = '';
    this.errMsg = '';
    this._render();
  }

  /**
   * Checks that all the required inputs (query and expires) have non-empty values. If so, it
   * returns true, otherwise, it will display an error on the element.
   */
  verifyFields(): boolean {
    if (this.query && this.expires) {
      this.errMsg = '';
      this._render();
      return true;
    }
    this.errMsg = 'Must specify a non-empty filter and an expiration.';
    this._render();
    return false;
  }
}

define('edit-ignore-rule-sk', EditIgnoreRuleSk);

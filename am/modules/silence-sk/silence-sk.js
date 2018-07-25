/**
 * @module /silence-sk
 * @description <h2><code>silence-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html/lib/lit-extended'
import { unsafeHTML } from 'lit-html/lib/unsafe-html'
import { $$ } from 'common-sk/modules/dom'
import { diffDate } from 'common-sk/modules/human'
import 'elements-sk/icon/delete-icon-sk'

const linkRe = /(http[s]?:\/\/[^\s]*)/gm;

function linkify(s) {
  console.log(s.replace(linkRe, '<a href="$&">$&</a>'));
  return s.replace(linkRe, '<a href="$&">$&</a>');
}

function notes(ele) {
  if (!ele._state.notes) {
    return [];
  }
  return ele._state.notes.map((note, index) => html`<section class=note>
  <p>${linkify(note.text)}</p>
  <div class=meta>
    <span class=author>${note.author}</span>
    <span class=date>${diffDate(note.ts*1000)}</span>
    <delete-icon-sk title='Delete comment.' on-click=${(e) => ele._deleteNote(e, index)}></delete-icon-sk>
  </div>
</section>`);
}

function table(o) {
  let keys = Object.keys(o);
  keys.sort();
  return keys.filter(k => !k.startsWith('__')).map((k) => html`<tr><th>${k}</th><td>${o[k].join(', ')}</td></tr>`);
}

const template = (ele) => html`
  <table>
    <tr><th>User:</th><td>${ele._state.user}</td></th>
    <tr><th>Duration:</th><td>${ele._state.duration}</td></th>
  </table>
  <table>
    ${table(ele._state.param_set)}
  </table>
  ${notes(ele)}
`;

window.customElements.define('silence-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  /** @prop state {Object} A Silence. */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  connectedCallback() {
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

});

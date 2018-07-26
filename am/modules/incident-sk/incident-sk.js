/**
 * @module incident-sk
 * @description <h2><code>incident-sk</code></h2>
 *
 * <p>
 *   Displays a single Incident.
 * </p>
 *
 * @attr closed {boolean} If not set then the incident is displayed in expanded
 *    mode, otherwise it is displayed in compact mode.
 *
 * @evt add-note Sent when the user adds a note to an incident.
 *    The detail includes the text of the note and the key of the incident.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *       text: "blah blah blah",
 *     }
 *   </pre>
 *
 * @evt del-note Sent when the user deletes a note on an incident.
 *    The detail includes the index of the note and the key of the incident.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *       index: 0,
 *     }
 *   </pre>
 *
 * @evt take Sent when the user wants the incident assigned to themselves.
 *    The detail includes the key of the incident.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *     }
 *   </pre>
 *
 * @evt assign Sent when the user want to assign the incident to someone else.
 *    The detail includes the key of the incident.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *     }
 *   </pre>
 *
 */
import { html, render } from 'lit-html/lib/lit-extended'
import { unsafeHTML } from 'lit-html/lib/unsafe-html'
import { $$ } from 'common-sk/modules/dom'
import { diffDate } from 'common-sk/modules/human'
import 'elements-sk/icon/delete-icon-sk'

const linkRe = /(http[s]?:\/\/[^\s]*)/gm;

function linkify(s) {
  return s.replace(linkRe, '<a href="$&">$&</a>');
}

function classOfH2(ele) {
  if (!ele._state.active) {
    return 'inactive'
  }
  if (ele._state.params.assigned_to) {
    return 'assigned'
  }
}

function abbr(ele) {
  let s = ele._state.params['abbr'];
  if (s) {
    return ` - ${s}`;
  } else {
    return ``
  }
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
  return keys.filter(k => !k.startsWith('__')).map((k) => html`<tr><th>${k}</th><td>${unsafeHTML(linkify(o[k]))}</td></tr>`);
}

function actionButtons(ele) {
  if (ele._state.active) {
    return html`<section class=assign>
      <button on-click=${e => ele._take(e)}>Take</button>
      <button on-click=${e => ele._assign(e)}>Assign</button>
    </section>`;
  } else {
    return html``;
  }
}

const template = (ele) => html`
  <h2 class$=${classOfH2(ele)} on-click=${e => ele._toggle(e)}>${ele._state.params.alertname} ${abbr(ele)}</h2>
  <section class=detail>
    ${actionButtons(ele)}
    <table>
      <tr><th>Started</th><td title=${new Date(ele._state.start*1000).toLocaleString()}>${diffDate(ele._state.start*1000)}</td></tr>
    ${table(ele._state.params)}
    </table>
    ${notes(ele)}
    <section>
      <textarea rows=2 cols=80></textarea>
      <button on-click=${(e) => ele._addNote(e)}>Submit</button>
    </section>
  </section>
`;

window.customElements.define('incident-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  /** @prop state {Object} An Incident. */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  _toggle(e) {
    if (this.hasAttribute('closed')) {
      this.removeAttribute('closed');
    } else {
      this.setAttribute('closed', 1);
    }
    this._render();
  }

  _take(e) {
    let detail = {
      key: this._state.key,
    };
    this.dispatchEvent(new CustomEvent('take', { detail: detail, bubbles: true }));
  }

  _assign(e) {
    let detail = {
      key: this._state.key,
    };
    this.dispatchEvent(new CustomEvent('assign', { detail: detail, bubbles: true }));
  }

  _deleteNote(e, index) {
    let detail = {
      key: this._state.key,
      index: index,
    };
    this.dispatchEvent(new CustomEvent('del-note', { detail: detail, bubbles: true }));
  }

  _addNote(e) {
    let textarea = $$('textarea', this);
    let detail = {
      key: this._state.key,
      text: textarea.value,
    };
    this.dispatchEvent(new CustomEvent('add-note', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _render() {
    render(template(this), this);
  }

});

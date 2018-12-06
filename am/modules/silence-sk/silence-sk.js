/**
 * @module /silence-sk
 * @description <h2><code>silence-sk</code></h2>
 *
 * @evt add-silence-note Sent when the user adds a note to an silence.
 *    The detail includes the text of the note and the key of the silence.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *       text: "blah blah blah",
 *     }
 *   </pre>
 *
 * @evt del-silence-note Sent when the user deletes a note on an silence.
 *    The detail includes the index of the note and the key of the silence.
 *
 *   <pre>
 *     detail {
 *       key: "12312123123",
 *       index: 0,
 *     }
 *   </pre>
 *
 * @evt save-silence Sent when the user saves a silence.
 *    The detail is the silence.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 * @evt archive-silence Sent when the user archives a silence.
 *    The detail is the silence.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 * @evt reactivate-silence Sent when the user reactivates a silence.
 *    The detail is the silence.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 * @evt delete-silence-param Sent when the user deletes a param from a silence.
 *    The detail is a copy of the silence with the parameter deleted.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 */
import 'elements-sk/icon/delete-icon-sk'

import * as paramset from '../paramset'
import { $$ } from 'common-sk/modules/dom'
import { abbr, displaySilence, expiresIn, notes } from '../am'
import { diffDate } from 'common-sk/modules/human'
import { html, render } from 'lit-html'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

function table(ele, o) {
  let keys = Object.keys(o);
  keys.sort();
  return keys.filter(k => !k.startsWith('__')).map((k) =>
    html`<tr><td><delete-icon-sk title='Delete rule.' @click=${(e) => ele._deleteRule(e, k)}></delete-icon-sk></td><th>${k}</th><td>${o[k].join(', ')}</td></tr>`);
}

function addNote(ele) {
  if (ele._state.key) {
    return html`
    <textarea rows=2 cols=80></textarea>
    <button @click=${ele._addNote}>Submit</button>
  `;
  } else {
    return html`<textarea rows=2 cols=80></textarea>`;
  }
}

function matches(ele) {
  if (!ele._incidents) {
    return ``;
  }
  return ele._incidents.filter(
    incident => paramset.match(ele._state.param_set, incident.params) && incident.active
  ).map(incident => html`<h2> ${incident.params.alertname} ${abbr(incident)}</h2>`);
}

function classOfH2(silence) {
  if (!silence.active) {
    return 'inactive';
  }
  return '';
}

function actionButtons(ele) {
  if (ele._state.active) {
    return html`<button @click=${ele._save}>Save</button>
                <button @click=${ele._archive}>Archive</button>`;
  } else {
    return html`<button @click=${ele._reactivate}>Reactivate</button>`;
  }
}

const template = (ele) => html`
  <h2 class=${classOfH2(ele._state)} @click=${ele._headerClick}>${displaySilence(ele._state)}</h2>
  <div class=body>
    <section class=actions>
      ${actionButtons(ele)}
    </section>
    <table class=info>
      <tr><th>User:</th><td>${ele._state.user}</td></th>
      <tr><th>Duration:</th><td><input @change=${ele._durationChange} value=${ele._state.duration}></input></td></th>
      <tr><th>Created</th><td title=${new Date(ele._state.created*1000).toLocaleString()}>${diffDate(ele._state.created*1000)}</td></tr>
      <tr><th>Expires</th><td>${expiresIn(ele._state)}</td></tr>
    </table>
    <table class=params>
      ${table(ele, ele._state.param_set)}
    </table>
    <section class=notes>
      ${notes(ele)}
    </section>
    <section class=addNote>
      ${addNote(ele)}
    </section>
    <section class=matches>
      <h1>Matches</h1>
      ${matches(ele)}
    </section>
  </div>
`;

window.customElements.define('silence-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = [];
  }

  connectedCallback() {
    upgradeProperty(this, 'state');
    upgradeProperty(this, 'incidents');
  }

  /** @prop state {Object} A Silence. */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  /** @prop incidents {string} The current active incidents. */
  get incidents() { return this._incidents }
  set incidents(val) {
    this._incidents = val;
    this._render();
  }

  _headerClick(e) {
    if (this.hasAttribute('collapsed')) {
      this.removeAttribute('collapsed');
    } else {
      this.setAttribute('collapsed', '');
    }
  }

  _durationChange(e) {
    this._state.duration = e.target.value;
  }

  _save(e) {
    let detail = {
      silence: this._state,
    };
    if (!this._state.key) {
      let textarea = $$('textarea', this);
      detail.silence.notes = [{
        text: textarea.value,
        ts: Math.floor(new Date().getTime()/1000),
      }]
    }
    this.dispatchEvent(new CustomEvent('save-silence', { detail: detail, bubbles: true }));
  }

  _archive(e) {
    let detail = {
      silence: this._state,
    };
    this.dispatchEvent(new CustomEvent('archive-silence', { detail: detail, bubbles: true }));
  }

  _reactivate(e) {
    let detail = {
      silence: this._state,
    };
    this.dispatchEvent(new CustomEvent('reactivate-silence', { detail: detail, bubbles: true }));
  }

  _deleteRule(e, key) {
    let silence = JSON.parse(JSON.stringify(this._state));
    delete silence.param_set[key];
    let detail = {
      silence: silence,
    };
    this.dispatchEvent(new CustomEvent('delete-silence-param', { detail: detail, bubbles: true }));
  }

  _addNote(e) {
    let textarea = $$('textarea', this);
    let detail = {
      key: this._state.key,
      text: textarea.value,
    };
    this.dispatchEvent(new CustomEvent('add-silence-note', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _deleteNote(e, index) {
    let detail = {
      key: this._state.key,
      index: index,
    };
    this.dispatchEvent(new CustomEvent('del-silence-note', { detail: detail, bubbles: true }));
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});

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
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import * as paramset from '../paramset'

const linkRe = /(http[s]?:\/\/[^\s]*)/gm;

function linkify(s) {
  return unsafeHTML(s.replace(linkRe, '<a href="$&">$&</a>'));
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

function table(ele, o) {
  let keys = Object.keys(o);
  keys.sort();
  return keys.filter(k => !k.startsWith('__')).map((k) =>
    html`<tr><td><delete-icon-sk title='Delete rule.' on-click=${(e) => ele._deleteRule(e, k)}></delete-icon-sk></td><th>${k}</th><td>${o[k].join(', ')}</td></tr>`);
}

function addNote(ele) {
  if (ele._state.key) {
    return html`
    <textarea rows=2 cols=80></textarea>
    <button on-click=${(e) => ele._addNote(e)}>Submit</button>
  `
  } else {
    return html`<textarea rows=2 cols=80></textarea>`
  }
}

function abbr(ele) {
  let s = ele.params['abbr'];
  if (s) {
    return ` - ${s}`;
  } else {
    return ``
  }
}

function matches(ele) {
  if (!ele._incidents) {
    return ``
  }
  return ele._incidents.filter(
    incident => paramset.match(ele._state.param_set, incident.params)
  ).map(incident => html`<h2> ${incident.params.alertname} ${abbr(incident)}</h2>`);

}

function displaySilence(silence) {
  let ret = [];
  for (let key in silence.param_set) {
    if (key.startsWith('__')) {
      continue
    }
    ret.push(`${key} - ${silence.param_set[key].join(', ')}`);
  }
  let s = ret.join(' ');
  if (s.length > 33) {
    s = s.slice(0, 30) + '...';
  }
  if (s.length == 0) {
    s = '(*)';
  }
  return s;
}

function classOfH2(silence) {
  if (!silence.active) {
    return 'inactive';
  }
  return '';
}

const template = (ele) => html`
  <h2 class$=${classOfH2(ele._state)} on-click=${e => ele._headerClick(e)}>${displaySilence(ele._state)}</h2>
  <div class=body>
    <section class=actions>
      <button on-click=${e => ele._save()}>Save</button>
      <button on-click=${e => ele._archive()}>Archive</button>
    </section>
    <table class=info>
      <tr><th>User:</th><td>${ele._state.user}</td></th>
      <tr><th>Duration:</th><td><input on-change=${e => ele._durationChange(e)} value=${ele._state.duration}></input></td></th>
      <tr><th>Created</th><td title=${new Date(ele._state.created*1000).toLocaleString()}>${diffDate(ele._state.created*1000)}</td></tr>
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
    render(template(this), this);
  }

});

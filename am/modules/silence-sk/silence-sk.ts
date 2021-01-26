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
 * @evt delete-silence Sent when the user deletes a silence.
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
 * @evt modify-silence-param Sent when the user modifies a param from a silence.
 *    The detail is a copy of the silence with the parameter modified.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 * @evt add-silence-param Sent when the user add a param to a silence.
 *    The detail is a copy of the silence with the new parameter added.
 *
 *   <pre>
 *     detail {
 *       silence: {...},
 *     }
 *   </pre>
 *
 */
import { define } from 'elements-sk/define';
import 'elements-sk/icon/add-box-icon-sk';
import 'elements-sk/icon/delete-icon-sk';

import { $$ } from 'common-sk/modules/dom';
import { diffDate } from 'common-sk/modules/human';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, render, TemplateResult } from 'lit-html';
import {
  abbr, displaySilence, expiresIn, getDurationTillNextDay, displayNotes,
} from '../am';
import * as paramset from '../paramset';
import { Incident, ParamSet, Note } from '../json';

const BOT_CENTRIC_PARAMS = ['alertname', 'bot'];

class State {
  key: string = '';

  param_set: ParamSet = {};

  duration: string = '';

  created: number = 0;

  user: string = '';

  notes: Note[] = [];

  active: boolean = false;
}

export class SilenceSk extends HTMLElement {
  notes: Note[] = [];

  // TODO(rmistry):
  // Will need a beter solution here
  private state: State = {
    key: '',
    param_set: {},
    duration: '',
    created: 0,
    user: '',
    notes: [],
    active: false,
  };

  private incidents: Incident[] = [];

  private static template = (ele: SilenceSk) => html`
  <h2 class=${ele.classOfH2()} @click=${ele._headerClick}>${displaySilence(ele)}</h2>
  <div class=body>
    <section class=actions>
      ${ele.actionButtons()}
    </section>
    <table class=info>
      <tr><th>User:</th><td>${ele.state.user}</td></th>
      <tr><th>Duration:</th><td><input class="duration" @change=${ele._durationChange} value=${ele.state.duration}></input><button class="param-btns" @click=${ele._tillNextShift}>Till next shift</button></td></th>
      <tr><th>Created</th><td title=${new Date(ele.state.created * 1000).toLocaleString()}>${diffDate(ele.state.created * 1000)}</td></tr>
      <tr><th>Expires</th><td>${expiresIn(ele)}</td></tr>
    </table>
    <table class=params>
      ${ele.table()}
    </table>
    <section class=notes>
      ${displayNotes(ele.state.notes, ele)}
    </section>
    <section class=addNote>
      ${ele.addNote()}
    </section>
    <section class=matches>
      <h1>Matches</h1>
      ${ele.displayMatches()}
    </section>
  </div>
`;

  // OK?
  connectedCallback(): void {
    this._render();
  }

  get silence_state(): State { return this.state; }

  set silence_state(val: State) {
    this.state = val;
    this._render();
  }

  /** @prop incidents The current active incidents. */
  get silence_incidents(): Incident[] { return this.incidents; }

  set silence_incidents(val: Incident[]) {
    this.incidents = val;
    this._render();
  }

  table(): TemplateResult[] {
    const keys = Object.keys(this.state.param_set);
    keys.sort();
    const botCentricParams = JSON.stringify(keys) === JSON.stringify(BOT_CENTRIC_PARAMS);
    const rules = keys.filter((k) => !k.startsWith('__')).map((k) => html`
      <tr>
        <td>
          <delete-icon-sk title='Delete rule.' @click=${() => this._deleteRule(k)}></delete-icon-sk>
        </td>
        <th>${k}</th>
        <td>
          <input class=param-val @change=${(e: Event) => this._modifyRule(e, k)} .value=${this.displayParamValue(this.state.param_set[k]!)}></input>
          ${this.displayAddBots(botCentricParams, k)}
        </td>
      </tr>`);
    rules.push(html`
      <tr>
        <td>
          <add-box-icon-sk title='Add rule.' @click=${() => this._addRule()}></add-box-icon-sk>
        </td>
        <td>
          <input id='add_param_key'></input>
        </td>
        <td>
          <input class=param-val id='add_param_value'></input>
        </td>
      </tr>
    `);
    return rules;
  }

  displayAddBots(botCentricParams: boolean, key: string): TemplateResult {
    if (botCentricParams && key === 'bot') {
      return html`<button class="param-btns" @click=${() => this._botsChooser()}>Add bot</button>`;
    }
    return html``;
  }

  displayParamValue(paramValue: string[]): string|string[] {
    if (paramValue.length > 1) {
      return `${paramValue.join('|')}`;
    }
    return paramValue;
  }

  addNote(): TemplateResult {
    if (this.state.key) {
      return html`
      <textarea rows=2 cols=80 placeholder="Add description for the silence"></textarea>
      <button @click=${this._addNote}>Submit</button>
    `;
    }
    return html`<textarea rows=2 cols=80 placeholder="Add description for the silence"></textarea>`;
  }

  gotoIncident(incident: Incident): void {
    window.location.href = `/?alert_id=${incident.id}&tab=1`;
  }

  displayMatches(): TemplateResult[] {
    if (!this.incidents) {
      return [];
    }
    return this.incidents.filter(
      (incident) => paramset.match(this.state.param_set, incident.params) && incident.active,
    ).map((incident) => html`<h2 @click=${() => this.gotoIncident(incident)}> ${incident.params.alertname} ${abbr(incident.params.abbr)}</h2>`);
  }

  classOfH2(): string {
    if (!this.state.active) {
      return 'inactive';
    }
    return '';
  }

  actionButtons(): TemplateResult {
    if (this.state.active) {
      return html`<button @click=${this._save}>Save</button>
                  <button @click=${this._archive}>Archive</button>`;
    }
    return html`<button @click=${this._reactivate}>Reactivate</button>
                  <delete-icon-sk title='Delete silence.' @click=${this._delete}></delete-icon-sk>`;
  }

  _headerClick(): void {
    if (this.hasAttribute('collapsed')) {
      this.removeAttribute('collapsed');
    } else {
      this.setAttribute('collapsed', '');
    }
  }

  _durationChange(e: Event): void {
    this.state.duration = (e.target as HTMLInputElement).value;
  }

  // Populates duration till next Monday 9am.
  _tillNextShift(): void {
    this.state.duration = getDurationTillNextDay(1, 9);
    this._render();
  }

  _save(): void {
    const detail = {
      silence: this.state,
    };
    if (!this.state.key) {
      const textarea = $$('textarea', this)! as HTMLInputElement;
      if (!textarea.value) {
        errorMessage('Please enter a description for the silence');
        textarea.focus();
        return;
      }
      detail.silence.notes = [{
        text: textarea.value,
        ts: Math.floor(new Date().getTime() / 1000),
        author: '', // The backend fills in the author.
      }];
    }
    this.dispatchEvent(new CustomEvent('save-silence', { detail: detail, bubbles: true }));
  }

  _archive(): void {
    const detail = {
      silence: this.state,
    };
    this.dispatchEvent(new CustomEvent('archive-silence', { detail: detail, bubbles: true }));
  }

  _reactivate(): void {
    const detail = {
      silence: this.state,
    };
    this.dispatchEvent(new CustomEvent('reactivate-silence', { detail: detail, bubbles: true }));
  }

  _delete(): void {
    const detail = {
      silence: this.state,
    };
    this.dispatchEvent(new CustomEvent('delete-silence', { detail: detail, bubbles: true }));
  }

  _deleteRule(key: string): void {
    const silence = JSON.parse(JSON.stringify(this.state));
    delete silence.param_set[key];
    const detail = {
      silence: silence,
    };
    this.dispatchEvent(new CustomEvent('delete-silence-param', { detail: detail, bubbles: true }));
  }

  _modifyRule(e: Event, key: string): void {
    const silence = JSON.parse(JSON.stringify(this.state));
    silence.param_set[key] = [(e.target as HTMLInputElement).value];
    const detail = {
      silence: silence,
    };
    this.dispatchEvent(new CustomEvent('modify-silence-param', { detail: detail, bubbles: true }));
  }

  _addRule(): void {
    const keyInput = $$('#add_param_key', this) as HTMLInputElement;
    if (!keyInput.value) {
      errorMessage('Please enter a name for the new param');
      keyInput.focus();
      return;
    }
    const valueInput = $$('#add_param_value', this) as HTMLInputElement;
    if (!valueInput.value) {
      errorMessage('Please enter a value for the new param');
      valueInput.focus();
      return;
    }

    // Dispatch event adding the new silence param.
    const silence = JSON.parse(JSON.stringify(this.state));
    silence.param_set[keyInput.value] = [valueInput.value];
    const detail = {
      silence: silence,
    };
    this.dispatchEvent(new CustomEvent('add-silence-param', { detail: detail, bubbles: true }));

    // Reset the manual param key and value.
    keyInput.value = '';
    valueInput.value = '';
  }

  _botsChooser(): void {
    this.dispatchEvent(new CustomEvent('bot-chooser', { detail: {}, bubbles: true }));
  }

  _addNote(): void {
    const textarea = $$('textarea', this) as HTMLInputElement;
    const detail = {
      key: this.state.key,
      text: textarea.value,
    };
    this.dispatchEvent(new CustomEvent('add-silence-note', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _deleteNote(e: Event, index: number): void {
    const detail = {
      key: this.state.key,
      index: index,
    };
    this.dispatchEvent(new CustomEvent('del-silence-note', { detail: detail, bubbles: true }));
  }

  _render(): void {
    render(SilenceSk.template(this), this, { eventContext: this });
  }
}

define('silence-sk', SilenceSk);

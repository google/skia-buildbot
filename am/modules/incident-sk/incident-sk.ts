/**
 * @module incident-sk
 * @description <h2><code>incident-sk</code></h2>
 *
 * <p>
 *   Displays a single Incident.
 * </p>
 *
 * @attr minimized {boolean} If not set then the incident is displayed in expanded
 *    mode, otherwise it is displayed in compact mode.
 *
 * @attr params {boolean} If set then the incident params are displayed, only
 *    applicable if minimized is true.
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
import { define } from 'elements-sk/define';
import 'elements-sk/icon/alarm-off-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/thumbs-up-down-icon-sk';
import '../silence-sk';

import { $$ } from 'common-sk/modules/dom';
import { diffDate, strDuration } from 'common-sk/modules/human';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, render, TemplateResult } from 'lit-html';
import { until } from 'lit-html/directives/until';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { abbr, linkify, displayNotes } from '../am';
import * as paramset from '../paramset';
import {
  Silence, Incident, Params, RecentIncidentsResponse, Note,
} from '../json';

const MAX_MATCHING_SILENCES_TO_DISPLAY = 50;

class State {
  key: string = '';

  id: string = '';

  params: Params = {};

  start: number = 0;

  last_seen: number = 0;

  active: boolean = false;

  notes: Note[] = [];
}


export class IncidentSk extends HTMLElement {
  private silences: Silence[] = [];

  private displaySilencesWithComments: boolean = false;

  private flaky: boolean = false;

  private recently_expired_silence: boolean = false;

  private state: State = {
    key: '',
    id: '',
    params: {},
    start: 0,
    last_seen: 0,
    active: false,
    notes: [],
  };

  private static template = (ele: IncidentSk) => html`
  <h2 class=${ele.classOfH2()}>${ele.state.params.alertname} ${abbr(ele.state.params.abbr)} ${ele._displayRecentlyExpired(ele.recently_expired_silence)} ${ele._displayFlakiness(ele.flaky)}</h2>
  <section class=detail>
    ${ele.actionButtons()}
    <table class=timing>
      <tr><th>Started</th><td title=${new Date(ele.state.start * 1000).toLocaleString()}>${diffDate(ele.state.start * 1000)}</td></tr>
      ${ele.lastSeen()}
      ${ele.duration()}
    </table>
    <table class=params>
      ${ele.table()}
    </table>
    ${displayNotes(ele.state.notes, ele)}
    <section class=addNote>
      <textarea rows=2 cols=80></textarea>
      <button @click=${ele._addNote}>Submit</button>
    </section>
    <section class=matchingSilences>
      <span class=matchingSilencesHeaders>
        <h3>Matching Silences</h3>
        <checkbox-sk ?checked=${ele.displaySilencesWithComments} @click=${ele._toggleSilencesWithComments} label="Show only silences with comments">
        </checkbox-sk>
      </span>
      ${ele.matchingSilences()}
    </section>
    <section class=history>
      <h3>History</h3>
      ${until(ele.history(), html`<div class=loading>Loading...</div>`)}
    </section>
  </section>
`;


  /** @prop state An Incident. */
  get incident_state(): State { return this.state; }

  set incident_state(val: State) {
    this.state = val;
    this._render();
  }

  /** @prop silences {string} The list of active silences. */
  get incident_silences(): Silence[] { return this.silences; }

  set incident_silences(val: Silence[]) {
    this._render();
    this.silences = val;
  }

  classOfH2(): string {
    if (!this.state.active) {
      return 'inactive';
    }
    if (this.state.params.assigned_to) {
      return 'assigned';
    }
    return '';
  }

  table(): TemplateResult[] {
    const params = this.state.params;
    const keys = Object.keys(params);
    keys.sort();
    return keys.filter((k) => !k.startsWith('__')).map((k) => html`<tr><th>${k}</th><td>${linkify(params[k])}</td></tr>`);
  }

  actionButtons(): TemplateResult {
    if (this.state.active) {
      let assignToOwnerButton = html``;
      if (this.state.params.owner) {
        assignToOwnerButton = html`<button @click=${this._assignToOwner}>Assign to Owner</button>`;
      }
      return html`<section class=assign>
        <button @click=${this._take}>Take</button>
        ${assignToOwnerButton}
        <button @click=${this._assign}>Assign</button>
      </section>`;
    }
    return html``;
  }

  matchingSilences(): TemplateResult[] {
    if (this.hasAttribute('minimized')) {
      return [];
    }
    // Filter out silences whose paramsets do not match and
    // which have no notes if displaySilencesWithComments is true.
    const filteredSilences = this.silences.filter((silence: Silence) => paramset.match(silence.param_set, this.state.params)
                                               && !(this.displaySilencesWithComments && this.doesSilenceHaveNoNotes(silence)));
    const ret = filteredSilences.slice(0, MAX_MATCHING_SILENCES_TO_DISPLAY).map((silence: Silence) => html`<silence-sk .silence_state=${silence} collapsable collapsed></silence-sk>`);
    if (!ret.length) {
      ret.push(html`<div class=nosilences>None</div>`);
    }
    return ret;
  }

  doesSilenceHaveNoNotes(silence: Silence): boolean {
    return !silence.notes || (silence.notes.length === 1 && silence.notes[0].text === '');
  }

  lastSeen(): TemplateResult {
    if (this.state.active) {
      return html``;
    }
    return html`<tr><th>Last Seen</th><td title=${new Date(this.state.last_seen * 1000).toLocaleString()}>${diffDate(this.state.last_seen * 1000)}</td></tr>`;
  }

  duration(): TemplateResult {
    if (this.state.active) {
      return html``;
    }
    return html`<tr><th>Duration</th><td>${strDuration(this.state.last_seen - this.state.start)}</td></tr>`;
  }

  history(): Promise<any> {
    if (this.hasAttribute('minimized') || this.state.id === '' || this.state.key === '') {
      return Promise.resolve();
    }
    return fetch(`/_/recent_incidents?id=${this.state.id}&key=${this.state.key}`, {
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'GET',
    }).then(jsonOrThrow).then((json: RecentIncidentsResponse) => {
      const incidents = json.incidents || [];
      this.flaky = json.flaky;
      this.recently_expired_silence = json.recently_expired_silence;
      // TODO(rmistry): Refresh needed?
      return incidents.map((i: Incident) => html`<incident-sk .incident_state=${i} minimized></incident-sk>`);
    }).catch(errorMessage);
  }

  _toggleSilencesWithComments(e: Event): void {
    // This prevents a double event from happening.
    e.preventDefault();
    this.displaySilencesWithComments = !this.displaySilencesWithComments;
    this._render();
  }

  _displayRecentlyExpired(recentlyExpiredSilence: boolean): TemplateResult {
    if (recentlyExpiredSilence) {
      return html`<alarm-off-icon-sk title='This alert has a recently expired silence'></alarm-off-icon-sk>`;
    }
    return html``;
  }

  _displayFlakiness(flaky: boolean): TemplateResult {
    if (flaky) {
      return html`<thumbs-up-down-icon-sk title='This alert is possibly flaky'></thumbs-up-down-icon-sk>`;
    }
    return html``;
  }

  _take(): void {
    const detail = {
      key: this.state.key,
    };
    this.dispatchEvent(new CustomEvent('take', { detail: detail, bubbles: true }));
  }

  _assignToOwner(): void {
    const detail = {
      key: this.state.key,
    };
    this.dispatchEvent(new CustomEvent('assign-to-owner', { detail: detail, bubbles: true }));
  }

  _assign(): void {
    const detail = {
      key: this.state.key,
    };
    this.dispatchEvent(new CustomEvent('assign', { detail: detail, bubbles: true }));
  }

  _deleteNote(e: Event, index: number): void {
    const detail = {
      key: this.state.key,
      index: index,
    };
    this.dispatchEvent(new CustomEvent('del-note', { detail: detail, bubbles: true }));
  }

  _addNote(): void {
    const textarea = $$('textarea', this) as HTMLInputElement;
    const detail = {
      key: this.state.key,
      text: textarea.value,
    };
    this.dispatchEvent(new CustomEvent('add-note', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _render(): void {
    if (!this.state) {
      return;
    }
    render(IncidentSk.template(this), this, { eventContext: this });
  }
}

define('incident-sk', IncidentSk);

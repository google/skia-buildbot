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
 *    applicable if minimzed is true.
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
import { html, render } from 'lit-html';
import { until } from 'lit-html/directives/until';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { abbr, linkify, notes } from '../am';
import * as paramset from '../paramset';

const MAX_MATCHING_SILENCES_TO_DISPLAY = 50;

function classOfH2(ele) {
  if (!ele._state.active) {
    return 'inactive';
  }
  if (ele._state.params.assigned_to) {
    return 'assigned';
  }
  return '';
}

function table(o) {
  const keys = Object.keys(o);
  keys.sort();
  return keys.filter((k) => !k.startsWith('__')).map((k) => html`<tr><th>${k}</th><td>${linkify(o[k])}</td></tr>`);
}

function actionButtons(ele) {
  if (ele._state.active) {
    let assignToOwnerButton = '';
    if (ele._state.params.owner) {
      assignToOwnerButton = html`<button @click=${ele._assignToOwner}>Assign to Owner</button>`;
    }
    return html`<section class=assign>
      <button @click=${ele._take}>Take</button>
      ${assignToOwnerButton}
      <button @click=${ele._assign}>Assign</button>
    </section>`;
  }
  return html``;
}

function matchingSilences(ele) {
  if (ele.hasAttribute('minimized')) {
    return '';
  }
  // Filter out silences whose paramsets do not match and
  // which have no notes if displaySilencesWithComments is true.
  const filteredSilences = ele._silences.filter((silence) => paramset.match(silence.param_set, ele._state.params)
                                             && !(ele._displaySilencesWithComments && doesSilenceHaveNoNotes(silence)));
  const ret = filteredSilences.slice(0, MAX_MATCHING_SILENCES_TO_DISPLAY).map((silence) => html`<silence-sk .state=${silence} collapsable collapsed></silence-sk>`);
  if (!ret.length) {
    ret.push(html`<div class=nosilences>None</div>`);
  }
  return ret;
}

function doesSilenceHaveNoNotes(silence) {
  return !silence.notes || (silence.notes.length === 1 && silence.notes[0].text === '');
}

function lastSeen(ele) {
  if (ele._state.active) {
    return '';
  }
  return html`<tr><th>Last Seen</th><td title=${new Date(ele._state.last_seen * 1000).toLocaleString()}>${diffDate(ele._state.last_seen * 1000)}</td></tr>`;
}

function duration(ele) {
  if (ele._state.active) {
    return '';
  }
  return html`<tr><th>Duration</th><td>${strDuration(ele._state.last_seen - ele._state.start)}</td></tr>`;
}

function history(ele) {
  if (ele.hasAttribute('minimized')) {
    return '';
  }
  return fetch(`/_/recent_incidents?id=${ele._state.id}&key=${ele._state.key}`, {
    headers: {
      'content-type': 'application/json',
    },
    credentials: 'include',
    method: 'GET',
  }).then(jsonOrThrow).then((json) => {
    json = json || {};
    const incidents = json.incidents || [];
    ele.flaky = json.flaky || false;
    ele.recently_expired_silence = json.recently_expired_silence || false;
    return incidents.map((i) => html`<incident-sk .state=${i} minimized></incident-sk>`);
  }).catch(errorMessage);
}

const template = (ele) => html`
  <h2 class=${classOfH2(ele)}>${ele._state.params.alertname} ${abbr(ele._state)} ${ele._displayRecentlyExpired(ele._recently_expired_silence)} ${ele._displayFlakiness(ele._flaky)}</h2>
  <section class=detail>
    ${actionButtons(ele)}
    <table class=timing>
      <tr><th>Started</th><td title=${new Date(ele._state.start * 1000).toLocaleString()}>${diffDate(ele._state.start * 1000)}</td></tr>
      ${lastSeen(ele)}
      ${duration(ele)}
    </table>
    <table class=params>
      ${table(ele._state.params)}
    </table>
    ${notes(ele)}
    <section class=addNote>
      <textarea rows=2 cols=80></textarea>
      <button @click=${ele._addNote}>Submit</button>
    </section>
    <section class=matchingSilences>
      <span class=matchingSilencesHeaders>
        <h3>Matching Silences</h3>
        <checkbox-sk ?checked=${ele._displaySilencesWithComments} @click=${ele._toggleSilencesWithComments} label="Show only silences with comments">
        </checkbox-sk>
      </span>
      ${matchingSilences(ele)}
    </section>
    <section class=history>
      <h3>History</h3>
      ${until(history(ele), html`<div class=loading>Loading...</div>`)}
    </section>
  </section>
`;

define('incident-sk', class extends HTMLElement {
  constructor() {
    super();
    this._silences = [];
    this._displaySilencesWithComments = false;
    this._flaky = false;
    this._recently_expired_silence = false;
  }

  /** @prop state {Object} An Incident. */
  get state() { return this._state; }

  set state(val) {
    this._state = val;
    this._render();
  }

  /** @prop silences {string} The list of active silences. */
  get silences() { return this._silences; }

  set silences(val) {
    this._render();
    this._silences = val;
  }

  /** @prop recently_expired_silence {bool} Whether silence recently expired. */
  get recently_expired_silence() { return this._recently_expired_silence; }

  set recently_expired_silence(val) {
    // No need to render again if value is same as old value.
    if (val !== this._recently_expired_silence) {
      this._recently_expired_silence = val;
      this._render();
    }
  }

  /** @prop flaky {bool} Whether this incident has been flaky. */
  get flaky() { return this._flaky; }

  set flaky(val) {
    // No need to render again if value is same as old value.
    if (val !== this._flaky) {
      this._flaky = val;
      this._render();
    }
  }

  _toggleSilencesWithComments(e) {
    // This prevents a double event from happening.
    e.preventDefault();
    this._displaySilencesWithComments = !this._displaySilencesWithComments;
    this._render();
  }

  _displayRecentlyExpired(recentlyExpiredSilence) {
    if (recentlyExpiredSilence) {
      return html`<alarm-off-icon-sk title='This alert has a recently expired silence'></alarm-off-icon-sk>`;
    }
    return '';
  }

  _displayFlakiness(flaky) {
    if (flaky) {
      return html`<thumbs-up-down-icon-sk title='This alert is possibly flaky'></thumbs-up-down-icon-sk>`;
    }
    return '';
  }

  _take() {
    const detail = {
      key: this._state.key,
    };
    this.dispatchEvent(new CustomEvent('take', { detail: detail, bubbles: true }));
  }

  _assignToOwner() {
    const detail = {
      key: this._state.key,
    };
    this.dispatchEvent(new CustomEvent('assign-to-owner', { detail: detail, bubbles: true }));
  }

  _assign() {
    const detail = {
      key: this._state.key,
    };
    this.dispatchEvent(new CustomEvent('assign', { detail: detail, bubbles: true }));
  }

  _deleteNote(e, index) {
    const detail = {
      key: this._state.key,
      index: index,
    };
    this.dispatchEvent(new CustomEvent('del-note', { detail: detail, bubbles: true }));
  }

  _addNote() {
    const textarea = $$('textarea', this);
    const detail = {
      key: this._state.key,
      text: textarea.value,
    };
    this.dispatchEvent(new CustomEvent('add-note', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _render() {
    if (!this._state) {
      return;
    }
    render(template(this), this, { eventContext: this });
  }
});

/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 * @attr csrf - The csrf string to attach to POST requests, based64 encoded.
 */
import 'elements-sk/checkbox-sk'
import 'elements-sk/error-toast-sk'
import 'elements-sk/icon/comment-icon-sk'
import 'elements-sk/icon/notifications-icon-sk'
import 'elements-sk/icon/person-icon-sk'
import 'elements-sk/spinner-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/tabs-panel-sk'
import 'elements-sk/tabs-sk'
import 'infra-sk/modules/login-sk'

import '../incident-sk'
import '../email-chooser-sk'
import '../silence-sk'

import { $$ } from 'common-sk/modules/dom'
import { Login } from 'infra-sk/modules/login'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import * as paramset from '../paramset'
import { abbr, displaySilence, expiresIn } from '../am'

// Legal states.
const START = 'start';
const INCIDENT = 'incident';
const EDIT_SILENCE = 'edit_silence';
const VIEW_STATS = 'view_stats';

function classOfH2(ele, incident) {
  let ret = [];
  if (!incident.active) {
    ret.push('inactive');
  } else if (incident.params.__silence_state === 'silenced') {
    ret.push('silenced');
  } else if (incident.params.assigned_to) {
    ret.push('assigned');
  }
  if (ele._selected && ele._selected.key === incident.key) {
    ret.push('selected');
  }
  return ret.join(' ');
}

function classOfSilenceH2(ele, silence) {
  var ret = [];
  if (!silence.active) {
    ret.push('inactive');
  }
  if (ele._selected && ele._selected.key === silence.key) {
    ret.push('selected');
  }
  return ret.join(' ');
}

function editIncident(ele) {
  if (ele._selected) {
    return html`<incident-sk .silences=${ele._silences} .state=${ele._selected}
      ></incident-sk>`
  } else {
    return ``
  }
}

function editSilence(ele) {
  return html`<silence-sk .state=${ele._current_silence} .incidents=${ele._incidents}
    ></silence-sk>`;
}

function viewStats(ele) {
  return ele._incident_stats.map((i, index) =>  html`<incident-sk .state=${i} ?minimized params=${index===0}></incident-sk>`)
}

function rightHandSide(ele) {
  switch (ele._state) {
    case START:
      return ``
    case INCIDENT:
      return editIncident(ele)
    case EDIT_SILENCE:
      return editSilence(ele)
    case VIEW_STATS:
      return viewStats(ele)
    default:
      return ``
  }
}

function hasNotes(o) {
  return (o.notes && o.notes.length > 0) ? '' : 'invisible';
}

function displayIncident(incident) {
  let ret = [incident.params.alertname];
  let abbr = incident.params['abbr'];
  if (abbr) {
    ret.push(` - ${abbr}`);
  }
  let s = ret.join(' ');
  if (s.length > 33) {
    s = s.slice(0, 30) + '...';
  }
  return s;
}

function trooper(ele) {
  if (ele._trooper === ele._user) {
    return html`<notifications-icon-sk title='You are the trooper, awesome!'></notifications-icon-sk>`
  }
  return ``
}

function assignedTo(incident, ele) {
  if (incident.params.assigned_to === ele._user) {
    return html`<person-icon-sk title='This item is assigned to you.'></person-icon-sk>`
  }
  return ``
}

function incidentList(ele, incidents) {
  return incidents.map(i => html`
    <h2 class=${classOfH2(ele, i)} @click=${e => ele._select(i)}>
    <span>
      <checkbox-sk ?checked=${ele._checked.has(i.key)} @change=${ele._check_selected} @click=${ele._suppress} id=${i.key}></checkbox-sk>
      ${assignedTo(i, ele)}
      ${displayIncident(i)}
    </span>
    <comment-icon-sk title='This incident has notes.' class=${hasNotes(i)}></comment-icon-sk>
    </h2>
    `)
}

function statsList(ele) {
  return ele._stats.map(stat => html`<h2 @click=${e => ele._statsClick(stat.incident)}>${displayIncident(stat.incident)} <span>${stat.num}</span></h2>`);
}

function numMatchSilence(ele, s) {
  if (!ele._incidents) {
    return ``;
  }
  return ele._incidents.filter(
    (incident) => paramset.match(s.param_set, incident.params) && incident.active
  ).length;
}

const template = (ele) => html`
<header>${trooper(ele)}<login-sk></login-sk></header>
<section class=nav>
  <tabs-sk @tab-selected-sk=${ele._tabSwitch}>
    <button>Mine</button>
    <button>Alerts</button>
    <button>Silences</button>
    <button>Stats</button>
  </tabs-sk>
  <tabs-panel-sk>
    <section class=mine>
      ${incidentList(ele, ele._incidents.filter(i => i.active && ((ele._user === ele._trooper && (i.params.__silence_state !== 'silenced')) || (i.params.assigned_to === ele._user))))}
    </section>
    <section class=incidents>
      ${incidentList(ele, ele._incidents)}
    </section>
    <section class=silences>
      ${ele._silences.map(i => html`
        <h2 class=${classOfSilenceH2(ele, i)} @click=${e => ele._silenceClick(i)}>
          <span>
            ${displaySilence(i)}
          </span>
          <span>
            <span title='Expires in'>${expiresIn(i)}</span>
            <comment-icon-sk title='This silence has notes.' class=${hasNotes(i)}></comment-icon-sk>
            <span title='The number of active alerts that match this silence.'>${numMatchSilence(ele, i)}</span>
          </span>
        </h2>`)}
    </section>
    <section class=stats>
      ${statsList(ele)}
    </section>
  </tabs-panel-sk>
</section>
<section class=edit>
  ${rightHandSide(ele)}
</section>
<footer>
  <spinner-sk id=busy></spinner-sk>
  <email-chooser-sk id=chooser></email-chooser-sk>
  <error-toast-sk></error-toast-sk>
<footer>
`;

function findParent(ele, tagName) {
  while (ele && (ele.tagName !== tagName)) {
    ele = ele.parentElement;
  }
  return ele;
}

window.customElements.define('alert-manager-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = []; // All active incidents.
    this._silences = []; // All active silences.
    this._stats = []; // Last requested stats.
    this._stats_range = '1w';
    this._incident_stats = []; // The incidents for a given stat.
    this._state = START; // One of START, INCIDENT, or EDIT_SILENCE.
    this._selected = null; // The selected incident, i.e. you clicked on the name.
    this._checked = new Set();    // Checked incidents, i.e. you clicked the checkbox.
    this._current_silence = null; // A silence under construction.
    this._ignored = [ '__silence_state', 'description', 'id', 'swarming', 'assigned_to']; // Params to ignore when constructing silences.
    this._user = 'barney@example.org';
    this._trooper = '';
    fetch('https://skia-tree-status.appspot.com/current-trooper?format=json', {mode: 'cors'}).then(jsonOrThrow).then(json => {
      this._trooper = json.username;
      this._render();
    });
    Login.then(loginstatus => {
      this._user = loginstatus.Email;
      this._render();
    });
  }

  connectedCallback() {
    this.addEventListener('save-silence', e => this._saveSilence(e.detail.silence));
    this.addEventListener('archive-silence', e => this._archiveSilence(e.detail.silence));
    this.addEventListener('reactivate-silence', e => this._reactivateSilence(e.detail.silence));
    this.addEventListener('add-silence-note', e => this._addSilenceNote(e));
    this.addEventListener('del-silence-note', e => this._delSilenceNote(e));
    this.addEventListener('delete-silence-param', e => this._deleteSilenceParam(e.detail.silence));
    this.addEventListener('add-note', e => this._addNote(e));
    this.addEventListener('del-note', e => this._delNote(e));
    this.addEventListener('take', e => this._take(e));
    this.addEventListener('assign', e => this._assign(e));

    this._render();
    this._busy = $$('#busy', this);
    this._favicon = $$('#favicon');

    this._busy.active = true;
    this._poll(true);
  }

  _poll(stopSpinner) {
    let incidents = fetch('/_/incidents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._incidents = json;
    });

    let silences = fetch('/_/silences', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._silences = json;
    });

    let emails = fetch('/_/emails', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._emails = json;
    });

    Promise.all([incidents, silences, emails]).then(() => { this._render() }).catch((msg) => {
      if (msg.resp) {
        msg.resp.text().then(errorMessage);
      } else {
        errorMessage(msg);
      }
    }).finally(() => {
      if (stopSpinner) {
        this._busy.active = false;
      }
      window.setTimeout(() => this._poll(), 10000)
    });
  }

  _tabSwitch(e) {
    // If tab is stats then load stats.
    if (e.detail.index === 3) {
      this._getStats();
    }
    this._state = START;
    this._render();
  }

  _suppress(e) {
    e.stopPropagation();
  }

  _silenceClick(silence) {
    this._current_silence = JSON.parse(JSON.stringify(silence));
    this._selected = silence;
    this._state = EDIT_SILENCE;
    this._render();
  }

  _statsClick(incident) {
    this._selected = incident;
    this._incidentStats();
    this._state = VIEW_STATS;
  }

  // Update the paramset for a silence as Incidents are checked and unchecked.
  _check_selected_impl(key, isChecked) {
    if (isChecked) {
      this._checked.add(key);
      this._incidents.forEach(i => {
        if (i.key == key) {
          paramset.add(this._current_silence.param_set, i.params, this._ignored);
        }
      });
    } else {
      this._checked.delete(key);
      this._current_silence.param_set = {};
      this._incidents.forEach(i => {
        if (this._checked.has(i.key)) {
          paramset.add(this._current_silence.param_set , i.params, this._ignored);
        }
      });
    }

    this._state = EDIT_SILENCE;
    this._render();
  }

  _check_selected(e) {
    let checkbox = findParent(e.target, 'CHECKBOX-SK');
    if (!this._checked.size) {
      // Request a new silence.
      fetch('/_/new_silence', {
        credentials: 'include',
      }).then(jsonOrThrow).then((json) => {
        this._selected = null;
        this._current_silence = json;
        // TODO(jcgregorio) Fix this once checkbox-sk is fixed.
        this._check_selected_impl(checkbox.id, checkbox._input.checked);
      }).catch(errorMessage);
    } else {
      // TODO(jcgregorio) Fix this once checkbox-sk is fixed.
      this._check_selected_impl(checkbox.id, checkbox._input.checked);
    }
  }

  _select(incident) {
    this._state = INCIDENT;
    this._checked = new Set();
    this._selected = incident;
    this._current_silence = null;
    this._render();
  }

  _addNote(e) {
    this._doImpl('/_/add_note', e.detail);
  }

  _delNote(e) {
    this._doImpl('/_/del_note', e.detail);
  }

  _deleteSilenceParam(silence) {
    // Don't save silences that are just being created when you delete a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, json => this._silenceAction(json, false));
  }

  _saveSilence(silence) {
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, json => this._silenceAction(json, true));
  }

  _archiveSilence(silence) {
    this._doImpl('/_/archive_silence', silence, json => this._silenceAction(json, true));
  }

  _reactivateSilence(silence) {
    this._doImpl('/_/reactivate_silence', silence, json => this._silenceAction(json, false));
  }

  _addSilenceNote(e) {
    this._doImpl('/_/add_silence_note', e.detail, json => this._silenceAction(json, false));
  }

  _delSilenceNote(e) {
    this._doImpl('/_/del_silence_note', e.detail, json => this._silenceAction(json, false));
  }

  _assign(e) {
    $$('#chooser', this).open(this._emails).then(email => {
      let detail = {
        key: e.detail.key,
        email: email,
      }
      this._doImpl('/_/assign', detail);
    });
  }

  _take(e) {
    this._doImpl('/_/take', e.detail);
  }

  _getStats() {
    let detail = {
      range: this._stats_range,
    }
    this._doImpl('/_/stats', detail, json => this._statsAction(json));
  }

  _incidentStats() {
    let detail = {
      incident: this._selected,
      range: this._stats_range,
    }
    this._doImpl('/_/incidents_in_range', detail, json => this._incidentStatsAction(json));
  }

  // Actions to take after updating incident stats.
  _incidentStatsAction(json) {
    this._incident_stats = json;
  }

  // Actions to take after updating Stats.
  _statsAction(json) {
    this._stats = json;
  }

  // Actions to take after updating an Incident.
  _incidentAction(json) {
    let incidents = this._incidents;
    for (let i = 0; i < incidents.length; i++) {
      if (incidents[i].key === json.key) {
        incidents[i] = json;
        break;
      }
    }
    this._selected = json;
  }

  // Actions to take after updating a Silence.
  _silenceAction(json, clear) {
    let found = false;
    this._current_silence = json;
    for (let i = 0; i < this._silences.length; i++) {
      if (this._silences[i].key === json.key) {
        this._silences[i] = json;
        found = true;
        break;
      }
    }
    if (!found) {
      this._silences.push(json);
    }
    if (clear) {
      this._state = START;
    }
  }


  // Common work done for all fetch requests.
  _doImpl(url, detail, action=json => this._incidentAction(json)) {
    this._busy.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': atob(this.getAttribute('csrf')),
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      action(json)
      this._render();
      this._busy.active = false;
    }).catch(msg => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  // Fix-up all the incidents and silences, including re-sorting them.
  _rationalize() {
    this._incidents.forEach(incident => {
      let silenced = this._silences.reduce((isSilenced, silence) => (isSilenced || (silence.active && paramset.match(silence.param_set, incident.params))), false);
      incident.params.__silence_state = silenced ? 'silenced' : 'active';
    });

    // Sort the incidents.
    let sortby = ['__silence_state', 'assigned_to', 'abbr', 'alertname', 'id'];
    this._incidents.sort((a,b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      // Inactive incidents are then sorted by 'lastseen' timestamp.
      if (!a.active) {
        let delta = b.last_seen - a.last_seen;
        if (delta) {
          return delta;
        }
      }
      for (let i = 0; i < sortby.length; i++) {
        let key = sortby[i];
        let left = a.params[key];
        let right = b.params[key];
        left = left || '';
        right = right || '';
        let cmp = left.localeCompare(right);
        if (cmp) {
          return cmp;
        }
      }
      return 0
    });
    this._silences.sort((a,b) => {
      // Sort active before inactive.
      if (a.active != b.active) {
        return a.active ? -1 : 1;
      }
      return b.updated - a.updated;
    });
  }

  _needsTriaging(incident, isTrooper) {
    if (incident.active
      && (incident.params.__silence_state != 'silenced')
      && (
        (isTrooper && !incident.params.assigned_to)
        || (incident.params.assigned_to == this._user)
      )
    ) {
      return true
    }
    return false
  }

  _render() {
    this._rationalize();
    render(template(this), this, {eventContext: this});
    // Update the icon.
    let isTrooper = this._user === this._trooper;
    let numActive = this._incidents.reduce((n, incident) => n += this._needsTriaging(incident, isTrooper) ? 1 : 0, 0);
    document.title = `${numActive} - AlertManager`;
    if (!this._favicon) {
      return
    }
    if (numActive > 0) {
      this._favicon.href = '/static/icon-active.png'
    } else {
      this._favicon.href = '/static/icon.png'
    }
  }

});

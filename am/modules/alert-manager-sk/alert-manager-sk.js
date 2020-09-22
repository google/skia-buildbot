/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */
import { define } from 'elements-sk/define';
import 'elements-sk/checkbox-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/alarm-off-icon-sk';
import 'elements-sk/icon/comment-icon-sk';
import 'elements-sk/icon/notifications-icon-sk';
import 'elements-sk/icon/person-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/toast-sk';

import '../incident-sk';
import '../email-chooser-sk';
import '../silence-sk';

import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, render } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { Login } from '../../../infra-sk/modules/login';

import * as paramset from '../paramset';
import { displaySilence, expiresIn } from '../am';

// Legal states.
const START = 'start';
const INCIDENT = 'incident';
const EDIT_SILENCE = 'edit_silence';
const VIEW_STATS = 'view_stats';

const MAX_SILENCES_TO_DISPLAY_IN_TAB = 50;

function classOfH2(ele, incident) {
  const ret = [];
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
  const ret = [];
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
      ></incident-sk>`;
  }
  return '';
}

function editSilence(ele) {
  return html`<silence-sk .state=${ele._current_silence} .incidents=${ele._incidents}
    ></silence-sk>`;
}

function viewStats(ele) {
  return ele._incident_stats.map((i, index) => html`<incident-sk .state=${i} ?minimized params=${index === 0}></incident-sk>`);
}

function rightHandSide(ele) {
  switch (ele._rhs_state) {
    case START:
      return '';
    case INCIDENT:
      return editIncident(ele);
    case EDIT_SILENCE:
      return editSilence(ele);
    case VIEW_STATS:
      return viewStats(ele);
    default:
      return '';
  }
}

function hasNotes(o) {
  return (o.notes && o.notes.length > 0) ? '' : 'invisible';
}

function hasRecentlyExpiredSilence(incident, idsToExpiredRecently) {
  return (idsToExpiredRecently[incident.id]) ? '' : 'invisible';
}

function displayIncident(incident) {
  const ret = [incident.params.alertname];
  const abbr = incident.params.abbr;
  if (abbr) {
    ret.push(` - ${abbr}`);
  }
  let s = ret.join(' ');
  if (s.length > 33) {
    s = `${s.slice(0, 30)}...`;
  }
  return s;
}

function trooper(ele) {
  if (ele._trooper === ele._user) {
    return html`<notifications-icon-sk title='You are the trooper, awesome!'></notifications-icon-sk>`;
  }
  return '';
}

function assignedTo(incident, ele) {
  if (incident.params.assigned_to === ele._user) {
    return html`<person-icon-sk title='This item is assigned to you.'></person-icon-sk>`;
  } if (incident.params.assigned_to) {
    return html`<span class='assigned-circle' title='This item is assigned to ${incident.params.assigned_to}.'>${incident.params.assigned_to[0].toUpperCase()}</span>`;
  }
  return '';
}

function incidentList(ele, incidents) {
  return incidents.map((i) => html`
    <h2 class=${classOfH2(ele, i)} @click=${() => ele._select(i)}>
    <span class=noselect>
      <checkbox-sk ?checked=${ele._checked.has(i.key)} @change=${ele._check_selected} @click=${ele._clickHandler} id=${i.key}></checkbox-sk>
      ${assignedTo(i, ele)}
      ${displayIncident(i)}
    </span>
    <span>
      <alarm-off-icon-sk title='This incident has a recently expired silence' class=${hasRecentlyExpiredSilence(i, ele._incidentsToRecentlyExpired)}></alarm-off-icon-sk>
      <comment-icon-sk title='This incident has notes.' class=${hasNotes(i)}></comment-icon-sk>
    </span>
    </h2>
    `);
}

function statsList(ele) {
  return ele._stats.map((stat) => html`<h2 @click=${() => ele._statsClick(stat.incident)}>${displayIncident(stat.incident)} <span>${stat.num}</span></h2>`);
}

function numMatchSilence(ele, s) {
  if (!ele._incidents) {
    return '';
  }
  return ele._incidents.filter(
    (incident) => paramset.match(s.param_set, incident.params) && incident.active,
  ).length;
}

function assignMultiple(ele) {
  return html`<button ?disabled=${ele._checked.size === 0} @click=${ele._assignMultiple}>Assign ${ele._checked.size} alerts</button>`;
}

const template = (ele) => html`
<header>${trooper(ele)}</header>
<section class=nav>
  <tabs-sk @tab-selected-sk=${ele._tabSwitch} selected=${ele._state.tab}>
    <button>Mine</button>
    <button>Alerts</button>
    <button>Silences</button>
    <button>Stats</button>
  </tabs-sk>
  <tabs-panel-sk>
    <section class=mine>
      ${assignMultiple(ele)}
      ${incidentList(ele, ele._incidents.filter((i) => i.active && i.params.__silence_state !== 'silenced' && (ele._user === ele._trooper || (i.params.assigned_to === ele._user) || (i.params.owner === ele._user && !i.params.assigned_to))))}
    </section>
    <section class=incidents>
      ${assignMultiple(ele)}
      ${incidentList(ele, ele._incidents)}
    </section>
    <section class=silences>
      ${ele._silences.slice(0, MAX_SILENCES_TO_DISPLAY_IN_TAB).map((i) => html`
        <h2 class=${classOfSilenceH2(ele, i)} @click=${() => ele._silenceClick(i)}>
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

define('alert-manager-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = []; // All active incidents.
    this._silences = []; // All active silences.
    this._stats = []; // Last requested stats.
    this._stats_range = '1w';
    this._incident_stats = []; // The incidents for a given stat.
    this._rhs_state = START; // One of START, INCIDENT, or EDIT_SILENCE.
    this._selected = null; // The selected incident, i.e. you clicked on the name.
    this._checked = new Set(); // Checked incidents, i.e. you clicked the checkbox.
    this._current_silence = null; // A silence under construction.
    // Params to ignore when constructing silences.
    this._ignored = ['__silence_state', 'description', 'id', 'swarming', 'assigned_to',
      'kubernetes_pod_name', 'instance', 'pod_template_hash', 'abbr_owner_regex',
      'controller_revision_hash'];
    this._shift_pressed_during_click = false; // If the shift key was held down during the mouse click.
    this._last_checked_incident = null; // Keeps track of the last checked incident. Used for multi-selecting incidents with shift.
    this._incidents_notified = {}; // Keeps track of all incidents that were notified via desktop notifications.
    this._incidentsToRecentlyExpired = {} // Map of incident IDs to whether their silences recently expired.
    this._user = 'barney@example.org';
    this._trooper = '';
    this._state = {
      tab: 0, // The selected tab.
      alert_id: '', // The selected alert (if any).
    };
    fetch('https://tree-status.skia.org/current-trooper', { mode: 'cors' }).then(jsonOrThrow).then((json) => {
      this._trooper = json.username;
      this._render();
    });
    Login.then((loginstatus) => {
      this._user = loginstatus.Email;
      this._render();
    });
  }

  connectedCallback() {
    this._requestDesktopNotificationPermission();

    this.addEventListener('save-silence', (e) => this._saveSilence(e.detail.silence));
    this.addEventListener('archive-silence', (e) => this._archiveSilence(e.detail.silence));
    this.addEventListener('reactivate-silence', (e) => this._reactivateSilence(e.detail.silence));
    this.addEventListener('delete-silence', (e) => this._deleteSilence(e.detail.silence));
    this.addEventListener('add-silence-note', (e) => this._addSilenceNote(e));
    this.addEventListener('del-silence-note', (e) => this._delSilenceNote(e));
    this.addEventListener('add-silence-param', (e) => this._addSilenceParam(e.detail.silence));
    this.addEventListener('delete-silence-param', (e) => this._deleteSilenceParam(e.detail.silence));
    this.addEventListener('modify-silence-param', (e) => this._modifySilenceParam(e.detail.silence));
    this.addEventListener('add-note', (e) => this._addNote(e));
    this.addEventListener('del-note', (e) => this._delNote(e));
    this.addEventListener('take', (e) => this._take(e));
    this.addEventListener('assign', (e) => this._assign(e));
    this.addEventListener('assign-to-owner', (e) => this._assignToOwner(e));

    this._stateHasChanged = stateReflector(
      () => this._state,
      (state) => {
        this._state = state;
        this._render();
      },
    );

    this._render();
    this._busy = $$('#busy', this);
    this._favicon = $$('#favicon');

    this._busy.active = true;
    this._poll(true);
  }

  _poll(stopSpinner) {
    const incidents = fetch('/_/incidents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._incidents = json.incidents;
      // If alert_id is specified and it is in supported rhs_states then display
      // an incident.
      if ((this._rhs_state == START || this._rhs_state == INCIDENT) &&
          this._state.alert_id) {
        for (let i = 0; i < this._incidents.length; i++) {
          if (this._incidents[i].id === this._state.alert_id) {
            this._select(this._incidents[i]);
            break;
          }
        }
      }
      this._incidents = json.incidents;
      this._incidentsToRecentlyExpired = json.ids_to_recently_expired_silences;
    });

    const silences = fetch('/_/silences', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._silences = json;
    });

    const emails = fetch('/_/emails', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._emails = json;
    });

    Promise.all([incidents, silences, emails]).then(() => { this._render(); }).catch((msg) => {
      if (msg.resp) {
        msg.resp.text().then(errorMessage);
      } else {
        errorMessage(msg);
      }
    }).finally(() => {
      if (stopSpinner) {
        this._busy.active = false;
      }
      window.setTimeout(() => this._poll(), 10000);
    });
  }

  _tabSwitch(e) {
    this._state.tab = e.detail.index;
    // Unset alert_id when switching tabs.
    this._state.alert_id = '';
    this._stateHasChanged();

    // If tab is stats then load stats.
    if (e.detail.index === 3) {
      this._getStats();
    }
    // If tab is silences then display empty silence to populate from scratch.
    // This will go away if any existing silence is clicked on.
    if (e.detail.index === 2) {
      fetch('/_/new_silence', {
        credentials: 'include',
      }).then(jsonOrThrow).then((json) => {
        this._selected = null;
        this._current_silence = json;
        this._rhs_state = EDIT_SILENCE;
        this._render();
      }).catch(errorMessage);
    } else {
      this._rhs_state = START;
      this._render();
    }
  }

  _clickHandler(e) {
    this._shift_pressed_during_click = e.shiftKey;
    e.stopPropagation();
  }

  _silenceClick(silence) {
    this._current_silence = JSON.parse(JSON.stringify(silence));
    this._selected = silence;
    this._rhs_state = EDIT_SILENCE;
    this._render();
  }

  _statsClick(incident) {
    this._selected = incident;
    this._incidentStats();
    this._rhs_state = VIEW_STATS;
  }

  // Update the paramset for a silence as Incidents are checked and unchecked.
  // TODO(jcgregorio) Remove this once checkbox-sk is fixed.
  _check_selected_impl(key, isChecked) {
    if (isChecked) {
      this._last_checked_incident = key;
      this._checked.add(key);
      this._incidents.forEach((i) => {
        if (i.key === key) {
          paramset.add(this._current_silence.param_set, i.params, this._ignored);
        }
      });
    } else {
      this._last_checked_incident = null;
      this._checked.delete(key);
      this._current_silence.param_set = {};
      this._incidents.forEach((i) => {
        if (this._checked.has(i.key)) {
          paramset.add(this._current_silence.param_set, i.params, this._ignored);
        }
      });
    }

    this._rhs_state = EDIT_SILENCE;
    this._render();
  }

  _check_selected(e) {
    const checkbox = findParent(e.target, 'CHECKBOX-SK');
    if (!this._checked.size) {
      // Request a new silence.
      fetch('/_/new_silence', {
        credentials: 'include',
      }).then(jsonOrThrow).then((json) => {
        this._selected = null;
        this._current_silence = json;
        this._check_selected_impl(checkbox.id, checkbox._input.checked);
      }).catch(errorMessage);
    } else if (this._shift_pressed_during_click && this._last_checked_incident) {
      let foundStart = false;
      let foundEnd = false;
      const incidents_to_check = [];
      this._incidents.some((i) => {
        if (i.key === this._last_checked_incident || i.key === checkbox.id) {
          if (!foundStart) {
            // This is the 1st time we have entered this block. This means we
            // found the first incident.
            foundStart = true;
          } else {
            // This is the 2nd time we have entered this block. This means we
            // found the last incident.
            foundEnd = true;
          }
        }
        if (foundStart) {
          incidents_to_check.push(i.key);
        }
        return foundEnd;
      });

      if (foundStart && foundEnd) {
        incidents_to_check.forEach((key) => {
          this._check_selected_impl(key, true);
        });
      } else {
        // Could not find start and/or end incident. Only check the last
        // clicked.
        this._check_selected_impl(checkbox.id, checkbox._input.checked);
      }
    } else {
      this._check_selected_impl(checkbox.id, checkbox._input.checked);
    }
  }

  _select(incident) {
    this._state.alert_id = incident.id;
    this._stateHasChanged();

    this._rhs_state = INCIDENT;
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

  _addSilenceParam(silence) {
    // Don't save silences that are just being created when you add a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _deleteSilenceParam(silence) {
    // Don't save silences that are just being created when you delete a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _modifySilenceParam(silence) {
    // Don't save silences that are just being created when you modify a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _saveSilence(silence) {
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, true));
  }

  _archiveSilence(silence) {
    this._doImpl('/_/archive_silence', silence, (json) => this._silenceAction(json, true));
  }

  _reactivateSilence(silence) {
    this._doImpl('/_/reactivate_silence', silence, (json) => this._silenceAction(json, false));
  }

  _deleteSilence(silence) {
    this._doImpl('/_/del_silence', silence, (json) => {
      for (let i = 0; i < this._silences.length; i++) {
        if (this._silences[i].key === json.key) {
          this._silences.splice(i, 1);
          this._rhs_state = START;
          break;
        }
      }
    });
  }

  _addSilenceNote(e) {
    this._doImpl('/_/add_silence_note', e.detail, (json) => this._silenceAction(json, false));
  }

  _delSilenceNote(e) {
    this._doImpl('/_/del_silence_note', e.detail, (json) => this._silenceAction(json, false));
  }

  _assign(e) {
    const owner = this._selected && this._selected.params.owner;
    $$('#chooser', this).open(this._emails, owner).then((email) => {
      const detail = {
        key: e.detail.key,
        email: email,
      };
      this._doImpl('/_/assign', detail);
    });
  }

  _assignMultiple() {
    const owner = (this._selected && this._selected.params.owner) || '';
    $$('#chooser', this).open(this._emails, owner).then((email) => {
      const detail = {
        keys: Array.from(this._checked),
        email: email,
      };
      this._doImpl('/_/assign_multiple', detail, (json) => {
        this._incidents = json;
        this._checked = new Set();
        this._render();
      });
    });
  }

  _assignToOwner(e) {
    const owner = this._selected && this._selected.params.owner;
    const detail = {
      key: e.detail.key,
      email: owner,
    };
    this._doImpl('/_/assign', detail);
  }

  _take(e) {
    this._doImpl('/_/take', e.detail);
    // Do not do desktop notification on takes, it is redundant.
    this._incidents_notified[e.detail.key] = true;
  }

  _getStats() {
    const detail = {
      range: this._stats_range,
    };
    this._doImpl('/_/stats', detail, (json) => this._statsAction(json));
  }

  _incidentStats() {
    const detail = {
      incident: this._selected,
      range: this._stats_range,
    };
    this._doImpl('/_/incidents_in_range', detail, (json) => this._incidentStatsAction(json));
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
    const incidents = this._incidents;
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
      this._rhs_state = START;
    }
  }

  // Common work done for all fetch requests.
  _doImpl(url, detail, action = (json) => this._incidentAction(json)) {
    this._busy.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then((json) => {
      action(json);
      this._render();
      this._busy.active = false;
    }).catch((msg) => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  // Fix-up all the incidents and silences, including re-sorting them.
  _rationalize() {
    this._incidents.forEach((incident) => {
      const silenced = this._silences.reduce((isSilenced, silence) => isSilenced
              || (silence.active && paramset.match(silence.param_set, incident.params)), false);
      incident.params.__silence_state = silenced ? 'silenced' : 'active';
    });

    // Sort the incidents, using the following 'sortby' list as tiebreakers.
    const sortby = ['__silence_state', 'assigned_to', 'alertname', 'abbr', 'id'];
    this._incidents.sort((a, b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      // Inactive incidents are then sorted by 'lastseen' timestamp.
      if (!a.active) {
        const delta = b.last_seen - a.last_seen;
        if (delta) {
          return delta;
        }
      }
      for (let i = 0; i < sortby.length; i++) {
        const key = sortby[i];
        const left = a.params[key] || '';
        const right = b.params[key] || '';
        const cmp = left.localeCompare(right);
        if (cmp) {
          return cmp;
        }
      }
      return 0;
    });
    this._silences.sort((a, b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      return b.updated - a.updated;
    });
  }

  _needsTriaging(incident, isTrooper) {
    if (incident.active
      && (incident.params.__silence_state !== 'silenced')
      && (
        (isTrooper && !incident.params.assigned_to)
        || (incident.params.assigned_to === this._user)
        || (incident.params.owner === this._user
            && !incident.params.assigned_to)
      )
    ) {
      return true;
    }
    return false;
  }

  _requestDesktopNotificationPermission() {
    if (Notification && Notification.permission === 'default') {
      Notification.requestPermission((permission) => {
        if (!('permission' in Notification)) {
          Notification.permission = permission;
        }
      });
    }
  }

  _sendDesktopNotification(unNotifiedIncidents) {
    if (unNotifiedIncidents.length === 0) {
      // Do nothing.
      return;
    }
    let text = '';
    if (unNotifiedIncidents.length === 1) {
      text = `${unNotifiedIncidents[0].params.alertname}\n\n${unNotifiedIncidents[0].params.description}`;
    } else {
      text = `There are ${unNotifiedIncidents.length} alerts assigned to you`;
    }
    const notification = new Notification('am.skia.org notification', {
      icon: '/static/icon-active.png',
      body: text,
      // 'tag' handles multi-tab scenarios. When multiple tabs are open then
      // only one notification is sent for the same alert.
      tag: `alertManagerNotification${text}`,
    });
    // onclick move focus to the am.skia.org tab and close the notification.
    const that = this;
    notification.onclick = function() {
      window.parent.focus();
      window.focus(); // Supports older browsers.
      that._select(unNotifiedIncidents[0]); // Display the 1st incident.
      this.close();
    };
    setTimeout(notification.close.bind(notification), 10000);
  }

  _render() {
    this._rationalize();
    render(template(this), this, { eventContext: this });
    // Update the icon.
    const isTrooper = this._user === this._trooper;
    const numActive = this._incidents.reduce((n, incident) => n += this._needsTriaging(incident, isTrooper) ? 1 : 0, 0);

    // Show desktop notifications only if permission was granted and only if
    // silences have been successfully fetched. If silences have not been
    // fetched yet then we might end up notifying on silenced incidents.
    if (Notification.permission === 'granted' && this._silences.length !== 0) {
      const unNotifiedIncidents = this._incidents.filter((i) => !this._incidents_notified[i.key] && this._needsTriaging(i, isTrooper));
      this._sendDesktopNotification(unNotifiedIncidents);
      unNotifiedIncidents.forEach((i) => this._incidents_notified[i.key] = true);
    }

    document.title = `${numActive} - AlertManager`;
    if (!this._favicon) {
      return;
    }
    if (numActive > 0) {
      this._favicon.href = '/static/icon-active.png';
    } else {
      this._favicon.href = '/static/icon.png';
    }
  }
});

/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for alert-manager.skia.org.
 *
 * @attr csrf - The csrf string to attach to POST requests, based64 encoded.
 */
import 'common-sk/modules/error-toast-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/spinner-sk'
import 'elements-sk/checkbox-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../incident-sk'
import '../email-chooser-sk'
import '../silence-sk'
import * as paramset from '../paramset'

// Legal states.
const START = 'start';
const INCIDENT = 'incident';
const EDIT_SILENCE = 'edit_silence';
const SILENCE = 'silence';

function classOfH2(ele, incident) {
  let ret = [];
  if (!incident.active) {
    ret.push('inactive');
  } else if (incident.params.assigned_to) {
    ret.push('assigned');
  }
  if (ele._selected && ele._selected.id == incident.id) {
    ret.push('selected');
  }
  return ret.join(' ');
}

function abbr(ele) {
  let s = ele.params['abbr'];
  if (s) {
    return ` - ${s}`;
  } else {
    return ``
  }
}

function editIncident(ele) {
  if (ele._selected) {
    return html`<incident-sk state=${ele._selected} on-add-note=${(e) => ele._addNote(e)} on-del-note=${(e) => ele._delNote(e)} on-take=${e => ele._take(e)} on-assign=${e => ele._assign(e)}></incident-sk>`
  } else {
    return ``
  }
}

function editSilence(ele) {
  return html`<silence-sk
    on-save-silence=${e => ele._saveSilence(e.detail.silence)}
    on-archive-silence=${e => ele._archiveSilence(e.detail.silence)}
    on-delete-silence-param=${e => ele._deleteSilenceParam(e.detail.key)}
    state=${ele._current_silence}></silence-sk>`;
}

function rightHandSide(ele) {
  switch (ele._state) {
    case START:
      return ``
    case INCIDENT:
      return editIncident(ele)
    case EDIT_SILENCE:
      return editSilence(ele)
    default:
      return ``
  }
}

const template = (ele) => html`
<section class=incidents>
  ${ele._incidents.map(i => html`
    <h2 class$=${classOfH2(ele, i)} on-click=${e => ele._select(i)}>
      <checkbox-sk checked?=${ele._checked.has(i.key)} on-change=${e => ele._check_selected(e)} on-click=${e => ele._suppress(e)} id=${i.key}></checkbox-sk> ${i.params.alertname} ${abbr(i)}
    </h2>
    `)}
  ${ele._recents.map(i => html`<h2 class$=${classOfH2(ele, i)} on-click=${e => ele._select(i)}>${i.params.alertname} ${abbr(i)}</h2>`)}
</section>
<section class=silences>
  ${ele._silences.map(i => html`<h2 on-click=${e => ele._silenceClick(i)}>${i.key}</h2>`)}
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
  while (ele != null && ele.tagName != tagName) {
    ele = ele.parentElement;
  }
  return ele;
}

window.customElements.define('alert-manager-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = []; // All active incidents.
    this._recents = [];  // Recently archived incidents.
    this._silences = []; // All active silences.

    this._state = START; // One of 'start', 'incident', 'edit_silence', 'silence'.
    this._columns = ['abbr'];
    this._selected = null; // The selected incident.
    this._checked = new Set();    // Checked incidents.
    this._current_silence = null; // A silence under construction.
    this._ignored = ['description', 'id', 'swarming', 'assigned_to'];
  }

  connectedCallback() {
    this._render();
    this._busy = $$('#busy', this);
    this._busy.active = true;
    fetch('/_/incidents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._incidents = json;
      this._render();
      this._busy.active = false;
    }).catch((msg) => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });

    fetch('/_/recents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._recents= json;
      this._render();
    }).catch((msg) => {
      msg.resp.text().then(errorMessage);
    });

    fetch('/_/silences', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._silences = json;
      this._render();
    }).catch((msg) => {
      msg.resp.text().then(errorMessage);
    });

    fetch('/_/emails', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._emails = json;
      this._render();
    }).catch(errorMessage);
  }

  _suppress(e) {
    e.stopPropagation();
  }

  _deleteSilenceParam(key) {
    delete this._current_silence.param_set[key];
    this._render();
  }

  _silenceClick(silence) {
    this._current_silence = silence;
    this._state = EDIT_SILENCE;
    this._render();
  }

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
    if (this._checked.size == 0) {
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

  _saveSilence(silence) {
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, json => this._silenceAction(json));
  }

  _archiveSilence(silence) {
    this._doImpl('/_/archive_silence', silence, json => this._silenceAction(json));
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

  _incidentAction(json) {
    let incidents = this._incidents;
    if (json.active == false) {
      incidents = this._recents;
    }
    for (let i = 0; i < incidents.length; i++) {
      if (incidents[i].key == json.key) {
        incidents[i] = json;
        break;
      }
    }
    this._selected = json;
  }

  _silenceAction(json) {
    let found = false;
    for (let i = 0; i < this._silences.length; i++) {
      if (this._silences[i].key == json.key) {
        if (json.active == true) {
          this._silences[i] = json;
        } else {
          this._silences.splice(i, 1);
        }
        found = true;
        break;
      }
    }
    if (!found) {
      this._silences.push(json);
    }
    this._state = START;
  }


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
//    }).catch(msg => {
//      this._busy.active = false;
//      msg.resp.text().then(errorMessage);
    });
  }

  _render() {
    let sortby = ['assigned_to', 'alertname'];
    sortby = sortby.concat(this._columns);
    this._incidents.sort((a,b) => {
      for (let i = 0; i < sortby.length; i++) {
        let key = sortby[i];
        let left = a.params[key];
        let right = b.params[key];
        left = left || '';
        right = right || '';
        let cmp = left.localeCompare(right);
        if (cmp != 0) {
          return cmp;
        }
      }
      return 0
    });
    render(template(this), this);
  }

});

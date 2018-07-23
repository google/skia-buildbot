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
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../incident-sk'
import '../email-chooser-sk'

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

const template = (ele) => html`
<main>
  <section class=incidents>
    ${ele._incidents.map(i => html`<h2 class$=${classOfH2(ele, i)} on-click=${e => ele._select(i)}>${i.params.alertname} ${abbr(i)}</h2>`)}
    ${ele._recents.map(i => html`<h2 class$=${classOfH2(ele, i)} on-click=${e => ele._select(i)}>${i.params.alertname} ${abbr(i)}</h2>`)}
  </section>
  <section class=edit>
    ${editIncident(ele)}
  </section>
</main>
<footer>
  <spinner-sk id=busy></spinner-sk>
  <email-chooser-sk id=chooser></email-chooser-sk>
  <error-toast-sk></error-toast-sk>
<footer>
`;

window.customElements.define('alert-manager-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = [];
    this._recents = [];
    this._columns = ["abbr"];
    this._selected;
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

    fetch('/_/emails', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._emails = json;
      this._render();
    }).catch(errorMessage);
  }

  _select(incident) {
    this._selected = incident;
    this._render();
  }

  _addNote(e) {
    this._doImpl("/_/add_note", e.detail);
  }

  _delNote(e) {
    this._doImpl("/_/del_note", e.detail);
  }

  _assign(e) {
    $$('#chooser', this).open(this._emails).then(email => {
      let detail = {
        key: e.detail.key,
        email: email,
      }
      this._doImpl("/_/assign", detail);
    });
  }

  _take(e) {
    this._doImpl("/_/take", e.detail);
  }

  _doImpl(url, detail) {
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
      // Should return with updated incident.
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
      this._render();
      this._busy.active = false;
    }).catch(msg => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  _render() {
    let sortby = ["assigned_to", "alertname"];
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

import 'elements-sk/styles/buttons';
import 'elements-sk/icon/alarm-icon-sk';
import 'elements-sk/icon/create-icon-sk';
import 'elements-sk/icon/warning-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/error-toast-sk';
import { errorMessage } from 'elements-sk/errorMessage';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../../../infra-sk/modules/systemd-unit-status-sk';
import '../../../infra-sk/modules/login-sk';

import { $$ } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';

import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';

import '../push-selection-sk';

// How often we should poll for status updates.
const UPDATE_MS = 5000;

// Utility functions for templating.
const monURI = (name) => `https://${name}-10000-proxy.skia.org`;
const logsURI = (name) => `https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=200&expandAll=false&resource=logging_log%2Fname%2F${name}`;
const prefixOf = (s) => s.split('/')[0];
const fullHash = (s) => s.slice(s.length - 44, s.length - 4);
const shorten = (s) => fullHash(s).slice(0, 6);

const alarmVisibility = (ele, installed) => {
  if (!ele._packageLookup[installed]) {
    return 'invisible';
  }
  return ele._packageLookup[installed].Latest ? 'invisible' : '';
};

const dirtyVisibility = (ele, installed) => {
  if (!ele._packageLookup[installed]) {
    return 'invisible';
  }
  return ele._packageLookup[installed].Dirty ? '' : 'invisible';
};

const logsFullURI = (name, installed) => {
  const app = installed.split('/')[0];
  return `https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=200&expandAll=false&resource=logging_log%2Fname%2F${name}&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2F${app}`;
};

const servicesOf = (ele, installed) => {
  const p = ele._packageLookup[installed];
  return p ? p.Services : [];
};

const listServices = (ele, server, installed) => servicesOf(ele, installed).map((service) => html`<systemd-unit-status-sk machine='${server.Name}' .value=${ele._state.status[`${server.Name}:${service}`]} ></systemd-unit-status-sk>`);

const listApplications = (ele, server) => server.Installed.map((installed) => html`
<div class=applicationRow>
  <button class=application data-server='${server.Name}' data-name='${installed}' data-app='${prefixOf(installed)}' @click=${ele._startChoose}><create-icon-sk title='Edit which package is installed.'></create-icon-sk></button>
  <warning-icon-sk class='${dirtyVisibility(ele, installed)}' title='Out of date.'></warning-icon-sk>
  <alarm-icon-sk class='${alarmVisibility(ele, installed)}' title='Uncommited changes when the package was built.'></alarm-icon-sk>
  <div class=serviceName><a href='https://github.com/google/skia-buildbot/compare/${fullHash(installed)}...HEAD'>${shorten(installed)}</a></div>
  <div class=logs><a href='${logsFullURI(server.Name, installed)}'>logs</a></div>
  <div>
    ${listServices(ele, server, installed)}
  </div>
  <div class=appName>${prefixOf(installed)}</div>
</div>`);

// Only display a server if it matches the current filter.
const classMatchFilter = (ele, server) => {
  const search = ele._query.search;
  // Short-circuit the most common case.
  if (!search) {
    return '';
  }
  return (server.Name.includes(search) || server.Installed.find((installed) => prefixOf(installed).includes(search))) ? '' : 'hidden';
};

const listServers = (ele) => ele._state.servers.map((server) => html`
<section class='${classMatchFilter(ele, server)}'>
  <h2>${server.Name}</h2>
  <button class=reboot raised data-action='start' data-name='reboot.target' data-server='${server.Name}' @click=${ele._reboot}>Reboot</button>
  [<a target=_blank href='${monURI(server.Name)}'>mon</a>]
  [<a target=_blank href='${logsURI(server.Name)}'>logs</a>]
  <div class=appContainer>
    ${listApplications(ele, server)}
  </div>
</section>`);

const template = (ele) => html`
<app-sk>
  <header><h1>Push</h1> <login-sk></login-sk></header>
  <main @unit-action=${(e) => ele._unitAction(e.detail)}>
    <section class=controls>
      <button id=refresh @click=${ele._refreshClick}>Refresh Packages</button>
      <spinner-sk id=spinner></spinner-sk>
      <label>Filter servers/apps: <input type=text @input=${ele._filterInput} value='${ele._query.search}'></input></label>
    </section>
    ${listServers(ele)}
  </main>
  <footer>
    <error-toast-sk></error-toast-sk>
    <push-selection-sk id='push-selection' @package-change=${ele._packageChange}></push-selection-sk>
    <confirm-dialog-sk id='confirm-dialog'></confirm-dialog-sk>
  </footer>
</app-sk>`;

/** <code>push-app-sk</code> custom element declaration.
 *
 * <p>
 *   The main element for the push application.
 * </p>
 */
class PushAppSk extends HTMLElement {
  constructor() {
    super();
    // Populated from push/main AllUI type.
    this._state = {
      servers: [],
      packages: {},
      status: {},
    };

    // Bits of state that get reflected to/from the URL query string.
    this._query = {
      // The current value of the filter text box.
      search: '',
    };
  }

  connectedCallback() {
    this._render();
    this._spinner = $$('#spinner');
    this._push_selection = $$('#push-selection');
    this._chosenServer = '';
    fetch('/_/state').then(jsonOrThrow).then((state) => {
      this._setState(state);
      this._updateStatus();
      this._render();
    }).catch(errorMessage);
    this._stateHasChanged = stateReflector(() => this._query, (query) => {
      this._query = query;
      this._render();
    });
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }

  // Called when the user presses the button to choose a different package version.
  // Presents a dialog of available package versions to choose from.
  _startChoose(e) {
    let target = e.target;
    if (target.nodeName !== 'BUTTON') {
      target = target.parentElement;
    }
    this._chosenServer = target.dataset.server;
    const choices = this._state.packages[target.dataset.app];
    const chosen = choices.findIndex((choice) => choice.Name === target.dataset.name);
    this._push_selection.choices = choices;
    this._push_selection.chosen = chosen;
    this._push_selection.show();
  }

  // Called when the user has actually made a selection from the dialog that
  // was displayed when _startChoose() was called.
  _packageChange(e) {
    this._push_selection.hide();
    this._spinner.active = true;
    const body = {
      name: e.detail.name,
      server: this._chosenServer,
    };
    fetch('/_/state', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
    }).then(jsonOrThrow).then((state) => {
      this._spinner.active = false;
      this._setState(state);
    }).catch((err) => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  _reboot(e) {
    const button = e.target;
    $$('#confirm-dialog').open(`Proceed with rebooting ${button.dataset.server}?`).then(() => {
      this._unitAction({
        machine: button.dataset.server,
        name: button.dataset.name,
        action: button.dataset.action,
      });
    });
  }

  // Perform an action on a systemd unit. The 'detail' must have a 'name',
  // 'action', and 'machine' properties.
  _unitAction(detail) {
    this._spinner.active = true;
    fetch(`/_/change?${fromObject(detail)}`, {
      method: 'POST',
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._spinner.active = false;
      errorMessage(json.result);
    }).catch((err) => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  // Set the new state of push.
  _setState(value) {
    this._state = value;
    this._packageLookup = {};
    for (const appName in this._state.packages) {
      let latest = true;
      this._state.packages[appName].forEach((details) => {
        this._packageLookup[details.Name] = details;
        this._packageLookup[details.Name].Latest = latest;
        latest = false;
      });
    }
    this._render();
  }

  // Get the new status from the push server.
  _updateStatus() {
    fetch('/_/status').then(jsonOrThrow).then((json) => {
      this._state.status = json;
      this._render();
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    }).catch((err) => {
      errorMessage(err);
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    });
  }

  // Refresh the full state from push, not just the status.
  _refreshClick(e) {
    this._spinner.active = true;
    fetch('/_/state?refresh=true').then(jsonOrThrow).then((json) => {
      this._setState(json);
      this._spinner.active = false;
    }).catch((err) => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  // Called when the user edits the filter text box.
  _filterInput(e) {
    this._query.search = e.target.value;
    this._stateHasChanged();
    this._render();
  }
}

define('push-app-sk', PushAppSk);

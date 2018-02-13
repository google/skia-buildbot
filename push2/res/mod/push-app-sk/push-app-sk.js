import 'skia-elements/buttons'
import 'skia-elements/spinner-sk'
import { $ } from 'skia-elements/dom'

import 'common/login-sk'
import 'common/error-toast-sk'
import 'common/systemd-unit-status-sk'
import { errorMessage } from 'common/errorMessage'
import { fromObject } from 'common/query'

import { html, render } from 'lit-html/lib/lit-extended'

import '../push-selection-sk'

const UPDATE_MS = 5000;

const monURI = (name) => `https://${name}-10000-proxy.skia.org`;
const logsURI = (name) => `https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=200&expandAll=false&resource=logging_log%2Fname%2F${name}`;
const prefixOf = (s) => s.split('/')[0];
const fullHash = (s) => s.slice(s.length-44, s.length-4);
const short = (s) => fullHash(s).slice(0, 6);
const alarmVisibility = (ele, installed) => {
  if (!ele._packageLookup[installed]) {
    return 'invisible'
  } else {
    return ele._packageLookup[installed].Latest ? 'invisible' : '';
  }
};
const dirtyVisibility = (ele, installed) => {
  if (!ele._packageLookup[installed]) {
    return 'invisible'
  } else {
    return ele._packageLookup[installed].Dirty ? '' : 'invisible';
  }
};
const logsFullURI = (name, installed) => {
  let app = installed.split('/')[0];
  return "https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=200&expandAll=false&resource=logging_log%2Fname%2F" + name + "&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2F" + app;
};

const servicesOf = (ele, installed) => {
  let p = ele._packageLookup[installed];
  if (!p) {
    return [];
  }
  return p.Services;
};

const listServices = (ele, server, installed) => servicesOf(ele, installed).map(service => {
  return html`<systemd-unit-status-sk machine$="${server.Name}" value=${ele._state.status[server.Name + ':' + service]} ></systemd-unit-status-sk>`;
});

const listApplications = (ele, server) => server.Installed.map(installed => html`
<div class=applicationRow>
  <button class=application data-server$="${server.Name}" data-name$="${installed}" data-app$="${prefixOf(installed)}" on-click=${e => ele._startChoose(e)}><icon-create-sk title="Edit which package is installed."></icon-create-sk></button>
  <icon-warning-sk class$="${dirtyVisibility(ele, installed)}" title="Out of date."></icon-warning-sk>
  <icon-alarm-sk class$="${alarmVisibility(ele, installed)}" title="Uncommited changes when the package was built."></icon-alarm-sk>
  <div class=serviceName><a href$="https://github.com/google/skia-buildbot/compare/${fullHash(installed)}...HEAD">${short(installed)}</a></div>
  <div><a href$="${logsFullURI(server.Name, installed)}">logs</a></div>
  <div>
    ${listServices(ele, server, installed)}
  </div>
  <div class=appName>${prefixOf(installed)}</div>
</div>`);


const matchSearch = (ele, server) => {
  return (server.Name.includes(ele._search) || server.Installed.find(installed => prefixOf(installed).includes(ele._search))) ? '' : 'hidden';
}

const listServers = (ele) => ele._state.servers.map(server => html`
<section class$="${matchSearch(ele, server)}">
  <h2>${server.Name}</h2>
  <button class=reboot raised data-action="start" data-name="reboot.target" data-server$="${server.Name}" on-click=${e => ele._reboot(e)}>Reboot</button>
  [<a target=_blank href$="${monURI(server.Name)}">mon</a>]
  [<a target=_blank href$="${logsURI(server.Name)}">logs</a>]
  <div>
    ${listApplications(ele, server)}
  </div>
</section>
`);

const template = (ele) => html`
<header><h1>Push</h1> <login-sk></login-sk></header>
<section class=controls>
  <button id=refresh on-click=${e => ele._refreshClick(e)}>Refresh Packages</button>
  <spinner-sk id=spinner></spinner-sk>
  <label>Filter servers/apps: <input type=text on-input=${e => ele._filterInput(e)}></input></label>
</section>
<main>
  ${listServers(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
  <push-selection-sk id="push-selection" on-package-change=${e => ele._packageChange(e)}></push-selection-sk>
</footer>
`;

const jsonOrThrow = (resp) => {
  if (resp.ok) {
    return resp.json();
  }
  throw 'Bad network response.';
}

// The <push-app-sk> custom element declaration.
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    None
//
window.customElements.define('push-app-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      servers: [],
      packages: {},
      status: {},
    };
    this._search = "";
  }

  connectedCallback() {
    fetch('/_/state').then(jsonOrThrow).then(this._setState).catch(errorMessage);
    this._updateStatus();
    this._render();
    this._spinner = $('spinner');
    this._push_selection = $('push-selection');
    this._chosenServer = '';
    this.addEventListener('unit-action', this._unitAction);
  }

  disconnectedCallback() {
    this.removeEventListener('unit-action', this._unitAction);
  }

  _startChoose(e) {
    let target = e.target;
    if (target.nodeName != 'BUTTON') {
      target = target.parentElement;
    }
    this._chosenServer = target.dataset.server;
    let choices = this._state.packages[target.dataset.app];
    let chosen = choices.findIndex(choice => choice.Name == target.dataset.name);
    this._push_selection.choices = choices;
    this._push_selection.chosen = chosen;
    this._push_selection.show();
  }

  _packageChange(e) {
    this._push_selection.hide();
    console.log(e);
    // TODO(jcgregorio) POST to /_/state and update element.
  }

  _reboot(e) {
    // TODO(jcgregorio) Confirm then call _unitAction().
  }

  _unitAction(e) {
    fetch("/_/change?" + fromObject(e.detail), { method: 'POST' }).
      then(jsonOrThrow).
      then(json => errorMessage(json.result)).
      catch(errorMessage);
  }

  _setState(value) {
    this._state = value;
    this._packageLookup = {};
    for (let appName in this._state.packages) {
      let latest = true;
      this._state.packages[appName].forEach(details => {
        this._packageLookup[details.Name] = details;
        this._packageLookup[details.Name].Latest = latest;
        latest = false;
      });
    }
    this._render();
  }

  _updateStatus() {
    fetch('/_/status').then(jsonOrThrow).then(json => {
      this._state.status = json;
      this._render();
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    }).catch(err => {
      errorMessage(err)
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    });
  }

  _render() {
    render(template(this), this);
  }

  _refreshClick(e) {
    this._spinner.active = true;
    fetch('/_/state?refresh=true').then(jsonOrThrow).then(json => {
      this._setState(json);
      this._spinner.active = false;
    }).catch(err => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  _filterInput(e) {
    // TODO(jcgregorio) Sync to URL.
    this._search = e.target.value;
    this._render();
  }

});

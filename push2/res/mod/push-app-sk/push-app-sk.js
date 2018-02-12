import 'skia-elements/buttons'
import 'skia-elements/spinner-sk'
import { $ } from 'skia-elements/dom'

import 'common/login-sk'
import 'common/error-toast-sk'
import 'common/systemd-unit-status-sk'
import { errorMessage } from 'common/errorMessage'

import { html, render } from 'lit-html/lib/lit-extended'

import '../push-selection-sk'

const UPDATE_MS = 5000;

const monURI = (name) => `https://${name}-10000-proxy.skia.org`;
const logsURI = (name) => `https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=200&expandAll=false&resource=logging_log%2Fname%2F${name}`;
const prefixOf = (s) => s.split('/')[0];

const listApplications = (ele, server) => server.Installed.map(installed => html`
  <button class=application data-server$="${server.Name}" data-name$="${installed}" data-app$="${prefixOf(installed)}"><icon-create-sk title="Edit which package is installed."></icon-create-sk></button>

  // Migrate
                <td><div class=appName>{{prefixOf(installed)}}</div></td>
                <td><span class=appName><a href$="https://github.com/google/skia-buildbot/compare/{{fullHash(installed)}}...HEAD">{{short(installed)}}</a></span></td>
                <td><iron-icon icon$="{{alarmIfNotLatest(installed)}}" title="Out of date."></iron-icon></td>
                <td><iron-icon icon$="{{warnIfDirty(installed)}}" title="Uncommited changes when the package was built."></iron-icon></td>
                <td><a href$="{{logsFullURI(server.Name,installed)}}">logs</a></td>
  <div>
  // listServices here.
  </div>
`;


const listServers = (ele) => ele.servers.map(server => html`
<section>
  <h2>${server.Name}</h2>
  <button raised data-action="start" data-name="reboot.target" data-server$="S{server.Name}">Reboot</button>
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
  <push-selection-sk></push-selection-sk>
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
  }

  connectedCallback() {
    fetch('/_/state').then(jsonOrThrow).then(json => {
      this._state = json;
      this._render();
    }).catch(errorMessage);
    this._updateStatus();
    this._render();
    this._spinner = $('spinner');
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
      this._spinner.active = false;
      this._state = json;
      this._render();
    }).catch(err => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  _filterInput(e) {
    console.log(e);
  }

});

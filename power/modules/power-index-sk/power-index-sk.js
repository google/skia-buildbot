import '../../../infra-sk/modules/app-sk';
import 'elements-sk/error-toast-sk';
import { errorMessage } from 'elements-sk/errorMessage';

import { diffDate } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';

// How often to update the data.
const UPDATE_INTERVAL_MS = 60000;

// Main template for this element
const template = (ele) => html`
<app-sk>
  <header><h1>Power Controller</h1></header>

  <main>
    <h2>Broken Bots (with powercycle support)</h2>

    ${downBotsTable(ele._bots, ele._hosts)}
  </main>
  <footer>
    <error-toast-sk></error-toast-sk>
  </footer>
<app-sk>`;

const downBotsTable = (bots, hosts) => html`
<table>
  <thead>
    <tr>
      <th>Name</th>
      <th>Key Dimensions</th>
      <th>Status</th>
      <th>Since</th>
      <th>Silenced</th>
    </tr>
  </thead>
  <tbody>
    ${listBots(bots)}
  </tbody>
</table>

<h2>Powercycle Commands</h2>
${listHosts(hosts, bots)}`;

const listBots = (bots) => bots.map((bot) => html`
<tr>
  <td>${bot.bot_id}</td>
  <td>${_keyDimension(bot)}</td>
  <td>${bot.status}</td>
  <td>${diffDate(bot.since)} ago</td>
  <td>${bot.silenced}</td>
</tr>`);

const listHosts = (hosts, bots) => hosts.map((host) => html`
<h3>On ${host}</h3>
<div class=code>${_command(host, bots)}</div>`);

// Helpers for templating
function _keyDimension(bot) {
  // TODO(kjlubick): Make this show only the important dimension.
  // e.g. for Android devices, just show "Nexus Player" or whatever
  if (!bot || !bot.dimensions) {
    return '';
  }
  let os = '';
  bot.dimensions.forEach((d) => {
    if (d.key === 'os') {
      os = d.value[d.value.length - 1];
    }
  });
  return os;
}

function _command(host, bots) {
  let hasBots = false;
  let cmd = 'powercycle ';
  for (const bot of bots) {
    if (bot.host_id === host && !bot.silenced) {
      hasBots = true;
      cmd += bot.bot_id;
      if (bot.status.startsWith('Device')) {
        cmd += '-device';
      }
      cmd += ' ';
    }
  }
  if (!hasBots) {
    return 'No unsilenced bots down :)';
  }
  return cmd;
}

// The <power-index-sk> custom element declaration.
//
//  This is the main page for power.skia.org.
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
define('power-index-sk', class extends HTMLElement {
  constructor() {
    super();
    this._hosts = [];
    this._bots = [];
  }

  connectedCallback() {
    this._render();
    // make a fetch ASAP, but not immediately (demo mock up may not be set up yet)
    window.setTimeout(() => this.update());
  }

  update() {
    fetch('/down_bots')
      .then(jsonOrThrow)
      .then((json) => {
        json.list = json.list || [];
        const byHost = {};
        json.list.forEach((b) => {
          const host_arr = byHost[b.host_id] || [];
          host_arr.push(b.bot_id);
          byHost[b.host_id] = host_arr;
        });
        json.list.sort((a, b) => a.bot_id.localeCompare(b.bot_id));
        this._bots = json.list;
        this._hosts = Object.keys(byHost);
        this._render();
        window.setTimeout(() => this.update(), UPDATE_INTERVAL_MS);
      })
      .catch((e) => {
        errorMessage(e);
        window.setTimeout(() => this.update(), UPDATE_INTERVAL_MS);
      });
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
});

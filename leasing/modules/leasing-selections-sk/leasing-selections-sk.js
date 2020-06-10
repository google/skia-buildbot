/**
 * @module module/leasing-selections-sk
 * @description <h2><code>leasing-selections-sk</code></h2>
 *
 * <p>
 *   Contains the selections the user needs to make to schedule a leasing
 *   swarming task.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import 'elements-sk/select-sk';
import 'elements-sk/styles/select';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { device, getAKAStr, doImpl } from '../leasing';

import '../../../infra-sk/modules/login-sk';

const _displayPools = (ele) => ele._all_pools.map(
  (p) => html`
    <option
      ?selected=${ele._pool === p}
      value=${p}
      title=${p}
      >${p}
    </option>`,
);

function _displayOsTypes(ele) {
  if (ele._osTypes === {}) {
    return '';
  }
  return Object.keys(ele._osTypes).map((o) => html`
    <option
      value=${o}
      title=${o}
      >${o} - ${ele._osTypes[o]} bots online
    </option>`);
}

function _displayDeviceTypes(ele) {
  if (ele._osToDeviceTypes === {}) {
    return '';
  }
  if (!(ele._selectedOsType in ele._osToDeviceTypes)) {
    return '';
  }
  const deviceTypes = ele._osToDeviceTypes[ele._selectedOsType];
  return Object.keys(deviceTypes).map((d) => html`
    <option
      value=${d}
      title=${d}
      >${device(d)} ${getAKAStr(d)} - ${deviceTypes[d]} bots online
    </option>`);
}

const template = (ele) => html`
    <table class="options panel">

      <tr>
        <td class="step-title">Select Pool</td>
        <td>
          <select id="pool" ?disabled=${ele._loadingDetails} .selection=${ele._pool} @input=${ele._poolChanged}>
            ${_displayPools(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select OS Type</td>
        <td>
          <select id="os_type" ?disabled=${ele._loadingDetails} @input=${ele._osTypeChanged}>
            ${_displayOsTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select Device Type</td>
        <td>
          <select id="device_type" ?disabled=${ele._loadingDetails || (ele._selectedOsType !== 'Android' && !ele._selectedOsType.startsWith('iOS'))}>
            ${_displayDeviceTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify BotId (optional)</td>
        <td>
          <input id="bot_id" ?disabled=${ele._loadingDetails}></input>
          <br/>
          <span class="smaller-font">Note: OS Type and Device Type are ignored if this is populated</span>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify Swarming Task Id<br/>to keep artifacts ready on bot<br/>(optional)</td>
        <td>
          <input id="task_id" ?disabled=${ele._loadingDetails}></input>
        </td>
      </tr>

      <tr>
        <td class="step-title">Lease Duration</td>
        <td>
          <select id="duration" ?disabled=${ele._loadingDetails}>
            <option value="1" title="1 Hour">1hr</option>
            <option value="2" title="2 Hours">2hr</option>
            <option value="6" title="6 Hours">6hr</option>
            <option value="23" title="23 Hours">23hr</option>
          </select>
        </td>

      <tr>
        <td class="step-title">Description</td>
        <td>
          <input id="desc" ?disabled=${ele._loadingDetails} placeholder="Description is required"></input>
        </td>
      </tr>

      <tr>
        <td colspan="2" class="center">
          <button raised @click=${ele._addTask} ?disabled=${ele._loadingDetails}>Lease Bot</button>
        </td>
      </tr>

    </table>
`;

define('leasing-selections-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._all_pools = ['Skia'];
    this._pool = 'Skia';
    this._osTypes = {};
    this._deviceTypes = {};

    this._selectedDeviceType = '';
    this._selectedOsType = '';
    this._osToDeviceTypes = {};

    // Maybe removable now because it's supposed to be fast???
    this._loadingDetails = false;

    this._fetchSupportedPools();
    this._fetchOsDetails();
  }

  _poolChanged(e) {
    this._pool = e.target.value;
    this._fetchOsDetails();
  }

  _osTypeChanged(e) {
    this._selectedOsType = e.target.value;
    this._render();
  }

  _fetchSupportedPools() {
    doImpl('/_/get_supported_pools', {}, (json) => {
      this._all_pools = json;
      this._render();
    });
  }

  _fetchOsDetails() {
    doImpl(`/_/pooldetails?pool=${this._pool}`, {}, (json) => {
      this._osTypes = json.OsTypes;
      this._osToDeviceTypes = json.OsToDeviceTypes;
      // Select first OsType.
      this._selectedOsType = Object.keys(this._osTypes)[0];
      this._render();
    });
  }

  _addTask() {
    const pool = $$('#pool', this).value;
    const osType = $$('#os_type', this).value;
    const deviceType = $$('#device_type', this).value;
    const botId = $$('#bot_id', this).value;
    const taskId = $$('#task_id', this).value;
    const duration = $$('#duration', this).value;
    const desc = $$('#desc', this).value;

    // Validate inputs.
    if (!desc) {
      errorMessage('Please specify a description');
      return;
    }

    // Confirm.
    const confirmed = window.confirm('Proceed with adding leasing task?');
    if (!confirmed) {
      return;
    }

    // Call backend to add task.
    const detail = {};
    detail.pool = pool;
    detail.botId = botId;
    if (!botId) {
      detail.osType = osType;
      if (deviceType) {
        detail.deviceType = deviceType;
      }
    }
    detail.taskIdForIsolates = taskId;
    detail.duration = duration;
    detail.description = desc;

    this._loadingDetails = true;
    this._render();
    doImpl('/_/add_leasing_task', detail, () => {
      window.location.href = '/my_leases';
    });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop appTitle {string} Reflects the app_title attribute for ease of use. */
  get appTitle() { return this.getAttribute('app_title'); }

  set appTitle(val) { this.setAttribute('app_title', val); }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});

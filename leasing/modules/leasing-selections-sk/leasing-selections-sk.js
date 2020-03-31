/**
 * @module module/leasing-selections-sk
 * @description <h2><code>leasing-selections-sk</code></h2>
 *
 * <p>
 *   Contains the title bar and error-toast for all the leasing server pages.
 *   The rest of pages should be a child of this element.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $, $$ } from 'common-sk/modules/dom';

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
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import '../../../infra-sk/modules/login-sk';

// Needs spinner?
// Needs confirm dialog. Look at vim am/modules/email-chooser-sk/email-chooser-sk.js for this.


const _displayPools = (ele) =>  ele._all_pools.map(
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
    </option>`,
  );
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
          <select id="os_type" ?disabled=${ele._loadingDetails} @selection-changed=${ele._osTypeChanged}>
            ${_displayOsTypes(ele)}
          </select>
        </td>
      </tr>

      <!-- LEFT HERE -->
      <tr>
        <td class="step-title">Select Device Type</td>
        <td>
          <select id="os_type" ?disabled=${ele._loadingDetails} @selection-changed=${ele._osTypeChanged}>
            ${_displayOsTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select Device Type</td>
        <td>
          <paper-dropdown-menu disabled="[[loadingDetails]]" id="device_dropdown_menu">
            <paper-listbox class="dropdown-content" attr-for-selected="value" id="device_listbox" selected="{{selectedDeviceType}}">
              <template is="dom-repeat" items="[[getKeys(deviceTypes)]]">
                <paper-item value="[[item]]">[[displayDeviceTypes(item, deviceTypes)]]</paper-item>
              </template>
            </paper-listbox>
          </paper-dropdown-menu>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify BotId (optional)</td>
        <td>
          <paper-input value="{{botId}}" disabled="[[loadingDetails]]"></paper-input>
          <span class="smaller-font">Note: OS Type and Device Type are ignored if this is populated</span>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify Swarming Task Id<br/>to keep artifacts ready on bot<br/>(optional)</td>
        <td>
          <paper-input value="{{taskIdForIsolates}}" disabled="[[loadingDetails]]"></paper-input>
        </td>
      </tr>

      <tr>
        <td class="step-title">Lease Duration</td>
        <td>
          <paper-dropdown-menu disabled="[[loadingDetails]]">
            <paper-listbox class="dropdown-content" selected="{{duration}}" attr-for-selected="value" id="duration_listbox">
              <paper-item value="1">1hr</paper-item>
              <paper-item value="2">2hr</paper-item>
              <paper-item value="6">6hr</paper-item>
              <paper-item value="23">23hr</paper-item>
            </paper-listbox>
          </paper-dropdown-menu>
        </td>

      <tr>
        <td class="step-title">Description</td>
        <td>
          <paper-input value="{{desc}}" label="Description is required" disabled="[[loadingDetails]]"></paper-input>
        </td>
      </tr>

      <tr>
        <td colspan="2" class="center">
          <paper-button raised id="submit" on-click="onSubmit">Lease Bot</paper-button>
        </td>
      </tr>

    </table>
`;

/**
 * Moves the elements from one NodeList to another NodeList.
 *
 * @param {NodeList} from - The list we are moving from.
 * @param {NodeList} to - The list we are moving to.
 */
function move(from, to) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

define('leasing-selections-sk', class extends ElementSk {

  constructor() {
    super(template)
    // TODO(rmistry): Get this list from the backend!;
    this._all_pools = ['Skia', 'SkiaCT', 'SkiaInternal', 'CT', 'CTAndroidBuilder', 'CTLinuxBuilder'];
    this._pool = 'Skia';
    this._osTypes = {};
    this._deviceTypes = {};

    this._selectedDeviceType = '';
    this._osToDeviceTypes = {};

    // Maybe removable now becaus it's supposed to be fast???
    this._loadingDetails = false;

    this._fetchOsDetails();
  }

  _poolChanged(e) {
    console.log("HERE HERE");
    this._pool = e.target.value;
    this._fetchOsDetails();
  }

  _fetchOsDetails() {
    this._doImpl('/_/pooldetails?pool=' + this._pool, {}, (json) => {           
      this._osTypes = json.OsTypes;                                             
      this._osToDeviceTypes = json.OsToDeviceTypes;                             
      console.log(json);                                                        
    });
  }

  // Common work done for all fetch requests.
  _doImpl(url, detail, action) {
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
    }).catch((msg) => {
      console.log("ERROR");
      console.log(msg);
      console.log(msg.resp);
      msg.resp.then(errorMessage);
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

/**
 * @module module/force-sync-instances-sk
 * @description <h2><code>force-sync-instances-sk</code></h2>
 *
 * <p>
 *   Displays all running Android compile backends, when they were last synced,
 *   and the frequency of their syncs. If the backends are currently syncing
 *   then it is displayed in the UI.
 *   Contains a "Force Sync All" button. The button signals to the backend that
 *   all instances should be synced.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { doImpl } from '../compile';

function formatTimestamp(timestamp) {
  if (!timestamp) {
    return timestamp;
  }
  const d = new Date(timestamp);
  return d.toLocaleString();
}

function getSyncStatus(inst) {
  if (inst.force_mirror_update) {
    return 'Currently Syncing';
  }
  return `Last synced at: ${formatTimestamp(inst.mirror_last_synced)}`;
}

function getCompileInstanceRows(ele) {
  return ele._compileInstances.map((inst) => html`
  <tr>
    <td>
      ${inst.name}
    </td>
    <td>
      ${getSyncStatus(inst)}
    </td>
    <td>
      Periodic syncs done every: ${inst.mirror_update_duration}
    </td>
  </tr>
  `);
}

const template = (ele) => html`
  <table class="forcesync">
    <tr class="headers">
       <td colspan=3>Instances</td>
    </tr>
    ${getCompileInstanceRows(ele)}
    <tr>
      <td colspan=3>
        <button raised @click=${ele._forceSync} ?disabled=${ele._areInstancesSyncing()}>Force Sync All</button>
      </td>
    </tr>
  </table>
`;

define('force-sync-instances-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._compileInstances = [];
    this._fetchCompileInstances();
  }

  _fetchCompileInstances() {
    doImpl('/_/compile_instances', {}, (json) => {
      this._compileInstances = json;
      this._render();
    });
  }

  _areInstancesSyncing() {
    let instancesSyncing = true;
    this._compileInstances.forEach((inst) => {
      if (!inst.force_mirror_update) {
        instancesSyncing = false;
      }
    });
    return instancesSyncing;
  }

  _forceSync() {
    const confirmed = window.confirm('Proceed with force syncing instances?');
    if (!confirmed) {
      return;
    }
    doImpl('/_/force_sync', {}, (json) => {
      this._compileInstances = json;
      this._render();
    });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});

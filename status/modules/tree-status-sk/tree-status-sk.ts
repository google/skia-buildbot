/**
 * @module modules/tree-status-sk
 * @description <h2><code>tree-status-sk</code></h2>
 *
 * Custom element for displaying tree status and tracking rotations.
 * @evt tree-status-update - Periodic event for updated tree-status and rotation information.
 *                           detail is of type TreeStatus.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { diffDate } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/android-icon-sk';
import 'elements-sk/icon/devices-other-icon-sk';

export interface Rotation {
  role: string;
  currentUrl: string;
  docLink: string;
  icon: string;
  name: string;
}
export interface TreeStatus {
  rotations: Array<Rotation>;
  status: TreeStatusResp;
}
// Type of rotations-update event.detail.
declare global {
  interface DocumentEventMap {
    'tree-status-update': CustomEvent<TreeStatus>;
  }
}

const treeStatusUrl = 'https://tree-status.skia.org/';

// Response structures from tree-status.skia.org.
// TODO(westont): Update once tree-status is migrated to generated TS.
export interface RoleResp {
  username?: string;
}
export interface TreeStatusResp {
  username?: string;
  date?: string;
  message?: string;
  general_state?: string;
}

export class TreeStatusSk extends ElementSk {
  private treeStatus: TreeStatus = {
    status: { message: 'Open', general_state: 'open' },
    rotations: [
      {
        role: 'Sheriff',
        currentUrl: `${treeStatusUrl}current-sheriff`,
        docLink: `${treeStatusUrl}sheriff`,
        icon: 'star',
        name: '',
      },
      {
        role: 'Wrangler',
        currentUrl: `${treeStatusUrl}current-wrangler`,
        docLink: `${treeStatusUrl}wrangler`,
        icon: 'gesture',
        name: '',
      },
      {
        role: 'Robocop',
        currentUrl: `${treeStatusUrl}current-robocop`,
        docLink: `${treeStatusUrl}robocop`,
        icon: 'android',
        name: '',
      },
      {
        role: 'Trooper',
        currentUrl: `${treeStatusUrl}current-trooper`,
        docLink: `${treeStatusUrl}trooper`,
        icon: 'devices-other',
        name: '',
      },
    ],
  };
  private static template = (el: TreeStatusSk) => html`
    <div>
      <span>
        <a href="https://tree-status.skia.org" target="_blank" rel="noopener noreferrer"
          >${el.treeStatus.status.message ? el.treeStatus.status.message : '(loading)'}</a
        >
      </span>
      <span class="nowrap">
        [${shortName(el.treeStatus.status.username)}
        ${el.treeStatus.status.date ? diffDate(el.treeStatus.status.date + 'UTC') : 'eons'} ago]
      </span>
    </div>
  `;

  constructor() {
    super(TreeStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    const fetches = (this.treeStatus.rotations.map((role) => {
      return fetch(role.currentUrl, { method: 'GET' })
        .then(jsonOrThrow)
        .then((json: RoleResp) => (role.name = shortName(json.username)))
        .catch(errorMessage);
    }) as Array<Promise<any>>).concat(
      fetch(`${treeStatusUrl}current`, { method: 'GET' })
        .then(jsonOrThrow)
        .then((json: TreeStatusResp) => (this.treeStatus.status = json))
        .catch(errorMessage)
    );

    Promise.all(fetches).finally(() => {
      this._render();
      this.dispatchEvent(
        new CustomEvent<TreeStatus>('tree-status-update', {
          bubbles: true,
          detail: this.treeStatus,
        })
      );
      window.setTimeout(() => this.refresh(), 60 * 1000);
    });
  }
}

function shortName(name?: string) {
  return name ? name.split('@')[0] : '';
}

define('tree-status-sk', TreeStatusSk);

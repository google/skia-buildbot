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
import { diffDate } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
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
const chopsRotationProxyUrl = 'https://chrome-ops-rotation-proxy.appspot.com/current/';

// This response structure comes from chrome-ops-rotation-proxy.appspot.com.
// We do not have access to the structure to generate TS.
export interface RoleResp {
  emails: string[];
}
// Response structure from tree-status.skia.org.
// TODO(westont): Update once tree-status is migrated to generated TS
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
        role: 'Skia',
        currentUrl: `${chopsRotationProxyUrl}grotation:skia-gardener`,
        docLink: 'https://rotations.corp.google.com/rotation/4699606003744768',
        icon: 'star',
        name: '',
      },
      {
        role: 'GPU',
        currentUrl: `${chopsRotationProxyUrl}grotation:skia-gpu-gardener`,
        docLink: 'https://rotations.corp.google.com/rotation/6176639586140160',
        icon: 'gesture',
        name: '',
      },
      {
        role: 'Android',
        currentUrl: `${chopsRotationProxyUrl}grotation:skia-android-gardener`,
        docLink: 'https://rotations.corp.google.com/rotation/5296436538245120',
        icon: 'android',
        name: '',
      },
      {
        role: 'Infra',
        currentUrl: `${chopsRotationProxyUrl}grotation:skia-infra-gardener`,
        docLink: 'https://rotations.corp.google.com/rotation/4617277386260480',
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
        ${el.treeStatus.status.date ? diffDate(`${el.treeStatus.status.date}UTC`) : 'eons'} ago]
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
    const fetches = (this.treeStatus.rotations.map((role) => fetch(role.currentUrl, { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: RoleResp) => {
        // Skia gardener rotations only have one entry.
        role.name = shortName(json.emails[0]);
      })
      .catch(errorMessage)) as Array<Promise<any>>).concat(
      fetch(`${treeStatusUrl}current`, { method: 'GET' })
        .then(jsonOrThrow)
        .then((json: TreeStatusResp) => (this.treeStatus.status = json))
        .catch(errorMessage),
    );

    Promise.all(fetches).finally(() => {
      this._render();
      this.dispatchEvent(
        new CustomEvent<TreeStatus>('tree-status-update', {
          bubbles: true,
          detail: this.treeStatus,
        }),
      );
      window.setTimeout(() => this.refresh(), 60 * 1000);
    });
  }
}

function shortName(name?: string) {
  return name ? name.split('@')[0] : '';
}

define('tree-status-sk', TreeStatusSk);

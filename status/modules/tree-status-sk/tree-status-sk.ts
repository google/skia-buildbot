/**
 * @module modules/tree-status-sk
 * @description <h2><code>tree-status-sk</code></h2>
 *
 * Custom element for displaying tree status and tracking rotations.
 * @evt tree-status-update - Periodic event for updated tree-status and rotation information.
 *                           detail is of type TreeStatus.
 *
 * @property baseURL: string - The base URL for getting tree status of specific repos.
 * @property repo: string - The repository we are currently looking at.
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
  private _baseURL: string = '';

  private _repo: string = '';

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
        <a href="${el.baseURL}/${el.repo}" target="_blank" rel="noopener noreferrer"
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

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('baseURL');
    this._upgradeProperty('repo');

    this.requestDesktopNotificationPermission();
    this._render();
    this.refresh();
  }

  private requestDesktopNotificationPermission(): void {
    if (Notification && Notification.permission === 'default') {
      Notification.requestPermission();
    }
  }

  private sendDesktopNotification(treeStatus: TreeStatusResp): void {
    // Do not notify if status window is already in focus.
    if (window.parent.document.hasFocus()) {
      return;
    }

    const msg = `${treeStatus.message} [${shortName(treeStatus.username)} ${treeStatus.date ? diffDate(`${treeStatus.date}UTC`) : 'eons'} ago]`;
    const notification = new Notification('Skia Tree Status Notification', {
      body: msg,
      // 'tag' handles multi-tab scenarios. When multiple tabs are open then
      // only one notification is sent for the same alert.
      tag: `statusNotification${treeStatus}`,
    });
    // onclick moves focus to the status tab and closes the notification.
    notification.onclick = () => {
      window.parent.focus();
      window.focus(); // Supports older browsers.
      notification.close();
    };
    setTimeout(notification.close.bind(notification), 10000);
  }

  private refresh() {
    if (!this.baseURL || !this.repo) {
      // Cannot refresh with baseURL or repo missing.
      return;
    }
    const fetches = (this.treeStatus.rotations.map((role) => fetch(role.currentUrl, { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: RoleResp) => {
        // Skia gardener rotations only have one entry.
        role.name = shortName(json.emails[0]);
      })
      .catch(errorMessage)) as Array<Promise<any>>).concat(
      fetch(`${this.baseURL}/${this.repo}/current`, { method: 'GET', credentials: 'include' })
        .then(jsonOrThrow)
        .then((json: TreeStatusResp) => {
          if (Notification.permission === 'granted' && json.message !== this.treeStatus.status.message) {
            // If the received message is different send a chrome notification.
            this.sendDesktopNotification(json);
          }
          this.treeStatus.status = json;
        })
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

  get baseURL(): string {
    return this._baseURL;
  }

  set baseURL(v: string) {
    this._baseURL = v;
    this._render();
    this.refresh();
  }

  get repo(): string {
    return this._repo;
  }

  set repo(v: string) {
    this._repo = v.toLowerCase();
    if (this._repo === 'infra') {
      // Special case: Status uses "infra" instead of "buildbot", but we need
      // the real repo name to fetch it's tree status.
      this._repo = 'buildbot';
    }
    this._render();
    this.refresh();
  }
}

function shortName(name?: string) {
  return name ? name.split('@')[0] : '';
}

define('tree-status-sk', TreeStatusSk);

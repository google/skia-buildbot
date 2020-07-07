/**
 * @module module/alerts-page-sk
 * @description <h2><code>alerts-page-sk</code></h2>
 *
 * A page for editing all the alert configs.
 */
import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/build-icon-sk';
import 'elements-sk/icon/create-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/styles/buttons';
import '../../../infra-sk/modules/paramset-sk';
import '../alert-config-sk';
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject, toParamSet } from 'common-sk/modules/query';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { Login } from '../../../infra-sk/modules/login';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { AlertConfigSk } from '../alert-config-sk/alert-config-sk';
import { FrameResponse, ParamSet, Alert, ConfigState } from '../json';
import { HintableObject } from 'common-sk/modules/hintable';

const okOrThrow = async (resp: Response) => {
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text);
  }
};

class AlertsPageSk extends ElementSk {
  private static template = (ele: AlertsPageSk) => html`
    <dialog>
      <alert-config-sk
        id="alertconfig"
        .paramset=${ele.paramset}
        .config=${ele.cfg}
      ></alert-config-sk>
      <div class="buttons">
        <button @click=${ele.cancel}>Cancel</button>
        <button @click=${ele.accept}>Accept</button>
      </div>
    </dialog>
    <table>
      <tr>
        <th></th>
        <th>Name</th>
        <th>Query</th>
        <th>Alert</th>
        <th>Owner</th>
        <th></th>
        <th></th>
        <th></th>
      </tr>
      ${AlertsPageSk.rows(ele)}
    </table>
    <div class="warning" ?hidden=${!!ele.alerts.length}>
      No alerts have been configured.
    </div>
    <button class="fab" @click=${ele.add} ?disabled=${!ele.email}>+</button>
    <checkbox-sk
      ?checked=${ele.showDeleted}
      @change=${ele.showChanged}
      label="Show deleted configs."
      id="showDeletedConfigs"
    ></checkbox-sk>
  `;

  private static rows = (ele: AlertsPageSk) =>
    ele.alerts.map(
      (item) => html`
    <tr>
      <td><create-icon-sk title='Edit' @click=${
        ele.edit
      } .__config=${item} ?disabled=${!ele.email}></create-icon-sk></td>
      <td>${item.display_name}</td>
      <td><paramset-sk .paramsets=${[
        toParamSet(item.query),
      ]}></paramset-sk></td>
      <td>${item.alert}</td>
      <td>${item.owner}</td>
      <td><delete-icon-sk title='Delete' @click=${
        ele.delete
      } .__config=${item} ?disabled=${!ele.email}></delete-icon-sk></td>
      <td> <a href=${AlertsPageSk.dryrunUrl(
        item
      )}> <build-icon-sk title='Dry Run'> </build-icon-sk> </td>
      <td>${AlertsPageSk.ifNotActive(item.state)}</td>
    </tr>
    `
    );

  private static dryrunUrl = (config: Alert) => {
    return `/d/?${fromObject((config as unknown) as HintableObject)}`;
  };

  private static ifNotActive(s: ConfigState) {
    return s === 'ACTIVE' ? '' : 'Archived';
  }

  private _cfg: Alert | null = null;

  private paramset: ParamSet = {};
  private alerts: Alert[] = [];
  private showDeleted: boolean = false;
  private email: string = '';
  private origCfg: Alert | null = null;
  private dialog: HTMLDialogElement | null = null;
  private alertconfig: AlertConfigSk | null = null;

  constructor() {
    super(AlertsPageSk.template);
    Login.then((status) => {
      this.email = status.Email;
      this._render();
    });
  }

  connectedCallback() {
    super.connectedCallback();
    const pInit = fetch('/_/initpage/')
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this.paramset = json.dataframe!.paramset;
      });
    const pList = this.listPromise().then((json) => {
      this.alerts = json;
    });
    Promise.all([pInit, pList])
      .then(() => {
        this._render();
        this.dialog = this.querySelector<HTMLDialogElement>('dialog');
        this.alertconfig = this.querySelector<AlertConfigSk>('#alertconfig');
        dialogPolyfill.registerDialog(this.dialog!);
        this.openOnLoad();
      })
      .catch(errorMessage);
  }

  private showChanged(e: InputEvent) {
    this.showDeleted = (e.target! as HTMLInputElement).checked;
    this.list();
  }

  /**
   * Start a request to get all the alerts.
   *
   * @returns {Promise} The started fetch().
   */
  private listPromise() {
    return fetch(`/_/alert/list/${this.showDeleted}`).then(jsonOrThrow);
  }

  /**
   * Load all the alerts from the server.
   */
  private list() {
    this.listPromise()
      .then((json: Alert[]) => {
        this.alerts = json;
        this._render();
        this.openOnLoad();
      })
      .catch(errorMessage);
  }

  /**
   * Display the modal dialog box for a specific alert if part of the URL.
   */
  private openOnLoad() {
    if (window.location.search.length === 0) {
      return;
    }
    const id = +window.location.search.slice(1);
    const matchingAlert = this.alerts.find((alert) => id === alert.id);
    if (matchingAlert) {
      this.startEditing(matchingAlert);
    }
    window.history.pushState(null, '', '/a/');
  }

  private add() {
    // Load an new Config from the server.
    fetch('/_/alert/new')
      .then(jsonOrThrow)
      .then((json: Alert) => {
        this.startEditing(json);
      })
      .catch(errorMessage);
  }

  private edit(e: MouseEvent) {
    if (!this.email) {
      errorMessage('You must be logged in to edit alerts.');
      return;
    }
    this.startEditing((e.target! as any).__config as Alert);
  }

  private startEditing(cfg: Alert) {
    this.origCfg = JSON.parse(JSON.stringify(this._cfg));
    this.cfg = cfg;
    this.dialog!.showModal();
  }

  private cancel() {
    this.dialog!.close();
  }

  private accept() {
    this.dialog!.close();
    this.cfg = this.alertconfig!.config;
    if (JSON.stringify(this.cfg) === JSON.stringify(this.origCfg)) {
      return;
    }
    // Post the config.
    fetch('/_/alert/update', {
      method: 'POST',
      body: JSON.stringify(this.cfg),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(okOrThrow)
      .then(() => {
        this.list();
      })
      .catch(errorMessage);
  }

  private delete(e: MouseEvent) {
    fetch(`/_/alert/delete/${((e.target! as any).__config as Alert).id}`, {
      method: 'POST',
    })
      .then(okOrThrow)
      .then(() => {
        this.list();
      })
      .catch(errorMessage);
  }

  /** The Alert being edited. */
  get cfg() {
    return this._cfg;
  }

  set cfg(val) {
    this._cfg = JSON.parse(JSON.stringify(val));
    if (this._cfg && !this._cfg.owner) {
      this._cfg.owner = this.email;
    }
    this._render();
  }
}

define('alerts-page-sk', AlertsPageSk);

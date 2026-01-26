/**
 * @module module/alerts-page-sk
 * @description <h2><code>alerts-page-sk</code></h2>
 *
 * A page for editing all the alert configs.
 */
import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/icons/delete-icon-sk';
import '../../../elements-sk/modules/icons/create-icon-sk';
import '../../../infra-sk/modules/paramset-sk';
import '../alert-config-sk';
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { fromObject, toParamSet } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { AlertConfigSk } from '../alert-config-sk/alert-config-sk';
import { FrameResponse, Alert, ConfigState, ReadOnlyParamSet } from '../json';
import { validate } from '../alert';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status } from '../../../infra-sk/modules/json';

const okOrThrow = async (resp: Response) => {
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text);
  }
};

export class AlertsPageSk extends ElementSk {
  private _cfg: Alert | null = null;

  private paramset = ReadOnlyParamSet({});

  private alerts: Alert[] = [];

  private showDeleted: boolean = false;

  private isEditor: boolean = false;

  private email: string = '';

  private origCfg: Alert | null = null;

  private dialog: HTMLDialogElement | null = null;

  private alertconfig: AlertConfigSk | null = null;

  constructor() {
    super(AlertsPageSk.template);
    void LoggedIn().then((value: Status) => {
      if (!value.roles) {
        return;
      }
      this.isEditor = value.roles.includes('editor');
      this.email = value.email;
      this._render();
    });
  }

  private static template = (ele: AlertsPageSk) => html`
    <dialog>
      <alert-config-sk
        id="alertconfig"
        .paramset=${ele.paramset}
        .config=${ele.cfg}></alert-config-sk>
      <div class="dialogButtons">
        <button class="cancel" @click=${ele.cancel}>Cancel</button>
        <button class="accept" @click=${ele.accept}>Accept</button>
      </div>
    </dialog>
    <button class="action" @click=${ele.add} ?disabled=${!ele.isEditor} title="Create a new alert.">
      New
    </button>
    <table id="alerts-table">
      <tr>
        <th></th>
        <th>Name</th>
        <th>Query</th>
        <th>${AlertsPageSk.alertOrComponentHeader()}</th>
        ${window.perf.need_alert_action === true ? html` <th>Action</th> ` : html``}
        <th>Owner</th>
        <th></th>
        <th></th>
        <th></th>
        <th></th>
      </tr>
      ${AlertsPageSk.rows(ele)}
    </table>
    <div class="warning" ?hidden=${!!ele.alerts.length}>No alerts have been configured.</div>
    <checkbox-sk
      ?checked=${ele.showDeleted}
      @change=${ele.showChanged}
      label="Show deleted configs."
      id="showDeletedConfigs"></checkbox-sk>
  `;

  private static displayIfAlertIsInvalid(item: Alert) {
    const msg = validate(item);
    if (msg === '') {
      return html``;
    }
    return html`${msg}`;
  }

  private static rows = (ele: AlertsPageSk) =>
    ele.alerts.map(
      (item) => html`
        <tr>
          <td>
            <create-icon-sk
              title="Edit"
              @click=${ele.edit}
              .__config=${item}
              ?disabled=${!ele.isEditor}></create-icon-sk>
          </td>
          <td>${item.display_name}</td>
          <td>
            <paramset-sk .paramsets=${[toParamSet(item.query)]}></paramset-sk>
          </td>
          <td>${AlertsPageSk.alertOrComponent(item)}</td>
          ${window.perf.need_alert_action === true
            ? html` <td>${item.action ?? 'noaction'}</td> `
            : html``}
          <td>${item.owner}</td>
          <td>${AlertsPageSk.displayIfAlertIsInvalid(item)}</td>
          <td>
            <delete-icon-sk
              title="Delete"
              @click=${ele.delete}
              .__config=${item}
              ?disabled=${!ele.isEditor}></delete-icon-sk>
          </td>
          <td><a href=${AlertsPageSk.dryrunUrl(item)}> Dry Run </a></td>
          <td>${AlertsPageSk.ifNotActive(item.state)}</td>
        </tr>
      `
    );

  private static alertOrComponent(item: Alert) {
    if (window.perf.notifications !== 'markdown_issuetracker') {
      return item.alert;
    }
    const issueTracker = 'https://issuetracker.google.com/issues?q=status:open%20componentid:';
    return html`<a href="${issueTracker}${item.issue_tracker_component}%26s=created_time:desc"
      >${item.issue_tracker_component}</a
    >`;
  }

  private static alertOrComponentHeader() {
    if (window.perf.notifications !== 'markdown_issuetracker') {
      return 'Alert';
    }
    return 'Component';
  }

  private static dryrunUrl = (config: Alert) =>
    `/d/?${fromObject(config as unknown as HintableObject)}`;

  private static ifNotActive(s: ConfigState) {
    return s === 'ACTIVE' ? '' : 'Archived';
  }

  connectedCallback(): void {
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
  private async listPromise() {
    return await fetch(`/_/alert/list/${this.showDeleted}`).then(jsonOrThrow);
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
    const id = window.location.search.slice(1);
    const matchingAlert = this.alerts.find((alert) => id === alert.id_as_string);
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
    if (!this.isEditor) {
      errorMessage('You must be logged in to edit alerts.');
      return;
    }
    this.startEditing((e.target! as any).__config as Alert);
  }

  private startEditing(cfg: Alert) {
    this.origCfg = structuredClone(this._cfg);
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
    fetch(`/_/alert/delete/${((e.target! as any).__config as Alert).id_as_string}`, {
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
    this._cfg = structuredClone(val);
    if (this._cfg && !this._cfg.owner) {
      this._cfg.owner = this.email;
    }
    this._render();
  }
}

define('alerts-page-sk', AlertsPageSk);

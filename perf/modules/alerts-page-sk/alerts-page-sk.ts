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
import { html, LitElement } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { fromObject, toParamSet } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { AlertConfigSk } from '../alert-config-sk/alert-config-sk';
import { FrameResponse, Alert, ConfigState, ReadOnlyParamSet } from '../json';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status } from '../../../infra-sk/modules/json';

const okOrThrow = async (resp: Response) => {
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text);
  }
};

@customElement('alerts-page-sk')
export class AlertsPageSk extends LitElement {
  @state()
  private _cfg: Alert | null = null;

  @state()
  private paramset = ReadOnlyParamSet({});

  @state()
  private alerts: Alert[] = [];

  @state()
  private showDeleted: boolean = false;

  @state()
  private isEditor: boolean = false;

  @state()
  private email: string = '';

  @state()
  private isDataLoaded: boolean = false;

  private origCfg: Alert | null = null;

  private dialog: HTMLDialogElement | null = null;

  private alertconfig: AlertConfigSk | null = null;

  constructor() {
    super();
    void LoggedIn().then((value: Status) => {
      if (!value.roles) {
        return;
      }
      this.isEditor = value.roles.includes('editor');
      this.email = value.email;
    });
  }

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <dialog>
        <alert-config-sk
          id="alertconfig"
          .paramset=${this.paramset}
          .config=${this.cfg}></alert-config-sk>
        <div class="dialogButtons">
          <button class="cancel" @click=${this.cancel}>Cancel</button>
          <button class="accept" @click=${this.accept}>Accept</button>
        </div>
      </dialog>
      <button
        class="action"
        @click=${this.add}
        ?disabled=${!this.isEditor}
        title="Create a new alert.">
        New
      </button>
      <table id="alerts-table">
        <tr>
          <th></th>
          <th>Name</th>
          <th>Query</th>
          <th>${this.alertOrComponentHeader()}</th>
          ${window.perf.need_alert_action === true ? html` <th>Action</th> ` : html``}
          <th>Owner</th>
          <th></th>
          <th></th>
          <th></th>
          <th></th>
        </tr>
        ${this.rows()}
      </table>
      <div class="warning" ?hidden=${!!this.alerts.length}>No alerts have been configured.</div>
      <checkbox-sk
        ?checked=${this.showDeleted}
        @change=${this.showChanged}
        label="Show deleted configs."
        id="showDeletedConfigs"></checkbox-sk>
    `;
  }

  private static validate(alert: Alert): string {
    if (!alert.query) {
      return 'An alert must have a non-empty query.';
    }
    return '';
  }

  private displayIfAlertIsInvalid(item: Alert) {
    const msg = AlertsPageSk.validate(item);
    if (msg === '') {
      return html``;
    }
    return html`${msg}`;
  }

  private rows() {
    return this.alerts.map(
      (item) => html`
        <tr>
          <td>
            <create-icon-sk
              title="Edit"
              @click=${this.edit}
              .__config=${item}
              ?disabled=${!this.isEditor}></create-icon-sk>
          </td>
          <td>${item.display_name}</td>
          <td>
            <paramset-sk .paramsets=${[toParamSet(item.query)]}></paramset-sk>
          </td>
          <td>${this.alertOrComponent(item)}</td>
          ${window.perf.need_alert_action === true
            ? html` <td>${item.action ?? 'noaction'}</td> `
            : html``}
          <td>${item.owner}</td>
          <td>${this.displayIfAlertIsInvalid(item)}</td>
          <td>
            <delete-icon-sk
              title="Delete"
              @click=${this.delete}
              .__config=${item}
              ?disabled=${!this.isEditor}></delete-icon-sk>
          </td>
          <td><a href=${this.dryrunUrl(item)}> Dry Run </a></td>
          <td>${this.ifNotActive(item.state)}</td>
        </tr>
      `
    );
  }

  private alertOrComponent(item: Alert) {
    if (window.perf.notifications !== 'markdown_issuetracker') {
      return item.alert;
    }
    const issueTracker = 'https://issuetracker.google.com/issues?q=status:open%20componentid:';
    return html`<a href="${issueTracker}${item.issue_tracker_component}%26s=created_time:desc"
      >${item.issue_tracker_component}</a
    >`;
  }

  private alertOrComponentHeader() {
    if (window.perf.notifications !== 'markdown_issuetracker') {
      return 'Alert';
    }
    return 'Component';
  }

  private dryrunUrl(config: Alert) {
    return `/d/?${fromObject(config as unknown as HintableObject)}`;
  }

  private ifNotActive(s: ConfigState) {
    return s === 'ACTIVE' ? '' : 'Archived';
  }

  connectedCallback() {
    super.connectedCallback();
    this.loadData();
  }

  private async loadData() {
    try {
      const pInit = fetch('/_/initpage/')
        .then(jsonOrThrow)
        .then((json: FrameResponse) => {
          this.paramset = json.dataframe!.paramset;
        });
      const pList = this.listPromise().then((json: Alert[]) => {
        this.alerts = json;
      });
      await Promise.all([pInit, pList]);
      this.isDataLoaded = true;
    } catch (e) {
      errorMessage(e as any);
    }
  }

  firstUpdated() {
    this.dialog = this.querySelector<HTMLDialogElement>('dialog');
    this.alertconfig = this.querySelector<AlertConfigSk>('#alertconfig');
  }

  updated(changedProperties: Map<string | symbol, unknown>) {
    if (changedProperties.has('isDataLoaded') && this.isDataLoaded) {
      this.openOnLoad();
    }
  }

  private showChanged(e: InputEvent) {
    this.showDeleted = (e.target! as HTMLInputElement).checked;
    this.list();
  }

  private async listPromise() {
    return await fetch(`/_/alert/list/${this.showDeleted}`).then(jsonOrThrow);
  }

  private list() {
    this.listPromise()
      .then((json: Alert[]) => {
        this.alerts = json;
        this.openOnLoad();
      })
      .catch(errorMessage);
  }

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
    if (this.alertconfig) {
      this.alertconfig.config = cfg;
    }
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

  private get cfg() {
    return this._cfg;
  }

  private set cfg(val) {
    this._cfg = structuredClone(val);
    if (this._cfg && !this._cfg.owner) {
      this._cfg.owner = this.email;
    }
  }
}

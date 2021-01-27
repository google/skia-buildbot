/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */
import { define } from 'elements-sk/define';
import 'elements-sk/checkbox-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/alarm-off-icon-sk';
import 'elements-sk/icon/comment-icon-sk';
import 'elements-sk/icon/notifications-icon-sk';
import 'elements-sk/icon/person-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/toast-sk';

import '../incident-sk';
import '../bot-chooser-sk';
import '../email-chooser-sk';
import '../silence-sk';

import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { HintableObject } from 'common-sk/modules/hintable';
import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, render, TemplateResult } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { Login } from '../../../infra-sk/modules/login';
import { BotChooserSk } from '../bot-chooser-sk/bot-chooser-sk';
import { EmailChooserSk } from '../email-chooser-sk/email-chooser-sk';

import * as paramset from '../paramset';
import { displaySilence, expiresIn } from '../am';

import {
  Silence, Incident, StatsRequest, Stat, IncidentsResponse, ParamSet, Params, IncidentsInRangeRequest,
} from '../json';

// Legal states.
const START = 'start';
const INCIDENT = 'incident';
const EDIT_SILENCE = 'edit_silence';
const VIEW_STATS = 'view_stats';

const MAX_SILENCES_TO_DISPLAY_IN_TAB = 50;

const BOT_CENTRIC_PARAMS = ['alertname', 'bot'];

class State {
  tab: number = 0; // The selected tab.

  alert_id: string = ''; // The selected alert (if any).
}

export class AlertManagerSk extends HTMLElement {
  // TODO(rmistry): REMOVE UNDERSCORES!

  private _incidents: Incident[] = []; // All active incidents.

  private _silences: Silence[] = []; // All active silences.

  private _stats: Stat[] = []; // Last requested stats.

  private _stats_range = '1w';

  private _incident_stats: Incident[] = []; // The incidents for a given stat.

  private _rhs_state = START; // One of START, INCIDENT, or EDIT_SILENCE.

  private _selected: Incident|Silence|null = null; // The selected incident, i.e. you clicked on the name.

  private _checked = new Set(); // Checked incidents, i.e. you clicked the checkbox.

  private _bots_to_incidents: Record<string, Incident[]> = {}; // Bot names to their incidents. Used in bot-centric view.

  private _isBotCentricView = false; // Determines if bot-centric view is displayed on incidents tab.

  private _current_silence: Silence|null = null; // A silence under construction.

  // Params to ignore when constructing silences.
  private _ignored = ['__silence_state', 'description', 'id', 'swarming', 'assigned_to',
    'kubernetes_pod_name', 'instance', 'pod_template_hash', 'abbr_owner_regex',
    'controller_revision_hash'];

  private _shift_pressed_during_click = false; // If the shift key was held down during the mouse click.

  private _last_checked_incident: string|null = null; // Keeps track of the last checked incident. Used for multi-selecting incidents with shift.

  private _incidents_notified: Record<string, boolean> = {}; // Keeps track of all incidents that were notified via desktop notifications.

  private _incidentsToRecentlyExpired: Record<string, boolean> = {}; // Map of incident IDs to whether their silences recently expired.

  private _user = 'barney@example.org';

  private _infra_gardener = '';

  // State is reflected to the URL via stateReflector.
  private _state: State = {
    tab: 0,
    alert_id: '',
  };

  private _favicon: HTMLAnchorElement | null = null;

  private spinner: SpinnerSk | null = null;

  private _emails: string[] = [];

  constructor() {
    super();

    fetch('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener', { mode: 'cors' }).then(jsonOrThrow).then((json) => {
      this._infra_gardener = json.emails[0];
      this._render();
    });
    Login.then((loginstatus) => {
      this._user = loginstatus.Email;
      this._render();
    });
  }

  private static template = (ele: AlertManagerSk) => html`
<header>${ele.infraGardener()}</header>
<section class=nav>
  <tabs-sk @tab-selected-sk=${ele._tabSwitch} selected=${ele._state.tab}>
    <button>Mine</button>
    <button>Alerts</button>
    <button>Silences</button>
    <button>Stats</button>
  </tabs-sk>
  <tabs-panel-sk>
    <section class=mine>
      <span class=selection-buttons>
        ${ele.assignMultiple()}
        ${ele.clearSelections()}
      </span>
      ${ele.incidentList(ele._incidents.filter((i: Incident) => i.active && i.params.__silence_state !== 'silenced' && (ele._user === ele._infra_gardener || (i.params.assigned_to === ele._user) || (i.params.owner === ele._user && !i.params.assigned_to))), false)}
    </section>
    <section class=incidents>
      ${ele.botCentricBtn()}
      <span class=selection-buttons>
        ${ele.assignMultiple()}
        ${ele.clearSelections()}
      </span>
      ${ele.incidentList(ele._incidents, ele._isBotCentricView)}
    </section>
    <section class=silences>
      ${ele._silences.slice(0, MAX_SILENCES_TO_DISPLAY_IN_TAB).map((i: Silence) => html`
        <h2 class=${ele.classOfSilenceH2(i)} @click=${() => ele._silenceClick(i)}>
          <span>
            ${displaySilence(i)}
          </span>
          <span>
            <span title='Expires in'>${expiresIn(i)}</span>
            <comment-icon-sk title='This silence has notes.' class=${ele.hasNotes(i)}></comment-icon-sk>
            <span title='The number of active alerts that match this silence.'>${ele.numMatchSilence(i)}</span>
          </span>
        </h2>`)}
    </section>
    <section class=stats>
      ${ele.statsList()}
    </section>
  </tabs-panel-sk>
</section>
<section class=edit>
  ${ele.rightHandSide()}
</section>
<footer>
  <spinner-sk id=busy></spinner-sk>
  <bot-chooser-sk id=bot-chooser></bot-chooser-sk>
  <email-chooser-sk id=email-chooser></email-chooser-sk>
  <error-toast-sk></error-toast-sk>
<footer>
`;


  connectedCallback(): void {
    this._requestDesktopNotificationPermission();

    this.addEventListener('save-silence', (e) => this._saveSilence((e as CustomEvent).detail.silence));
    this.addEventListener('archive-silence', (e) => this._archiveSilence((e as CustomEvent).detail.silence));
    this.addEventListener('reactivate-silence', (e) => this._reactivateSilence((e as CustomEvent).detail.silence));
    this.addEventListener('delete-silence', (e) => this._deleteSilence((e as CustomEvent).detail.silence));
    this.addEventListener('add-silence-note', (e) => this._addSilenceNote(e as CustomEvent));
    this.addEventListener('del-silence-note', (e) => this._delSilenceNote(e as CustomEvent));
    this.addEventListener('add-silence-param', (e) => this._addSilenceParam((e as CustomEvent).detail.silence));
    this.addEventListener('delete-silence-param', (e) => this._deleteSilenceParam((e as CustomEvent).detail.silence));
    this.addEventListener('modify-silence-param', (e) => this._modifySilenceParam((e as CustomEvent).detail.silence));
    this.addEventListener('add-note', (e) => this._addNote(e as CustomEvent));
    this.addEventListener('del-note', (e) => this._delNote(e as CustomEvent));
    this.addEventListener('take', (e) => this._take(e as CustomEvent));
    this.addEventListener('bot-chooser', () => this._botChooser());
    this.addEventListener('assign', (e) => this._assign(e as CustomEvent));
    this.addEventListener('assign-to-owner', (e) => this._assignToOwner(e as CustomEvent));

    this.stateHasChanged = stateReflector(
      /* getState */ () => (this._state as unknown) as HintableObject,
      /* setState */ (newState) => {
        this._state = (newState as unknown) as State;
        this._render();
      },
    );

    this._render();
    this.spinner = $$('#busy', this) as SpinnerSk;
    this._favicon = $$('#favicon');

    this.spinner.active = true;
    this._poll(true);
  }

  classOfH2(incident: Incident): string {
    const ret = [];
    if (!incident.active) {
      ret.push('inactive');
    } else if (incident.params.__silence_state === 'silenced') {
      ret.push('silenced');
    } else if (incident.params.assigned_to) {
      ret.push('assigned');
    }
    if (this._selected && this._selected.key === incident.key) {
      ret.push('selected');
    }
    return ret.join(' ');
  }

  classOfSilenceH2(silence: Silence): string {
    const ret = [];
    if (!silence.active) {
      ret.push('inactive');
    }
    if (this._selected && this._selected.key === silence.key) {
      ret.push('selected');
    }
    return ret.join(' ');
  }

  editIncident(): TemplateResult {
    if (this._selected) {
      return html`<incident-sk .incident_silences=${this._silences} .incident_state=${this._selected}
        ></incident-sk>`;
    }
    return html``;
  }

  editSilence(): TemplateResult {
    return html`<silence-sk .silence_state=${this._current_silence} .silence_incidents=${this._incidents}
      ></silence-sk>`;
  }

  viewStats(): TemplateResult[] {
    // HERE HERE - params-= ?
    return this._incident_stats.map((i, index) => html`<incident-sk .incident_state=${i} ?minimized params=${index === 0}></incident-sk>`);
  }

  rightHandSide(): TemplateResult|TemplateResult[] {
    switch (this._rhs_state) {
      case START:
        return [];
      case INCIDENT:
        return this.editIncident();
      case EDIT_SILENCE:
        return this.editSilence();
      case VIEW_STATS:
        return this.viewStats();
      default:
        return [];
    }
  }

  hasNotes(o: Incident| Silence): string {
    return (o.notes && o.notes.length > 0) ? '' : 'invisible';
  }

  hasRecentlyExpiredSilence(incident: Incident): string {
    return (this._incidentsToRecentlyExpired[incident.id]) ? '' : 'invisible';
  }

  displayIncident(incident: Incident): string {
    const ret = [incident.params.alertname];
    const abbr = incident.params.abbr;
    if (abbr) {
      ret.push(` - ${abbr}`);
    }
    let s = ret.join(' ');
    if (s.length > 33) {
      s = `${s.slice(0, 30)}...`;
    }
    return s;
  }

  infraGardener(): TemplateResult {
    if (this._infra_gardener === this._user) {
      return html`<notifications-icon-sk title='You are the Infra Gardener, awesome!'></notifications-icon-sk>`;
    }
    return html``;
  }

  assignedTo(incident: Incident): TemplateResult {
    if (incident.params.assigned_to === this._user) {
      return html`<person-icon-sk title='This item is assigned to you.'></person-icon-sk>`;
    } if (incident.params.assigned_to) {
      return html`<span class='assigned-circle' title='This item is assigned to ${incident.params.assigned_to}.'>${incident.params.assigned_to[0].toUpperCase()}</span>`;
    }
    return html``;
  }

  populateBotsToIncidents(incidents: Incident[]): void {
    // Reset bots_to_incidents and populate it from scratch.
    this._bots_to_incidents = {};
    for (let i = 0; i < incidents.length; i++) {
      const incident = incidents[i];
      if (incident.params && incident.params.bot) {
        // Only consider active bot incidents that are not assigned or silenced.
        if (!incident.active || incident.params.__silence_state === 'silenced'
            || incident.params.assigned_to) {
          continue;
        }
        const botName = incident.params.bot;
        if (this._bots_to_incidents[botName]) {
          this._bots_to_incidents[botName].push(incident);
        } else {
          this._bots_to_incidents[botName] = [incident];
        }
      }
    }
  }

  botCentricView(): TemplateResult[] {
    this.populateBotsToIncidents(this._incidents);
    const botsHTML: TemplateResult[] = [];
    Object.keys(this._bots_to_incidents).forEach((botName) => {
      botsHTML.push(html`
        <h2 class="bot-centric">
          <span class=noselect>
            <checkbox-sk class=bot-alert-checkbox ?checked=${this.isBotChecked(this._bots_to_incidents[botName])} @change=${this._check_selected} @click=${this._clickHandler} id=${botName}></checkbox-sk>
            <span class=bot-alert>
              ${botName}
              <span class=bot-incident-list>
                ${this.incidentListForBot(this._bots_to_incidents[botName])}
              </span>
            </span>
          </span>
        </h2>
      `);
    });
    return botsHTML;
  }

  // Checks to see if all the incidents for the bot are checked.
  isBotChecked(incidents: Incident[]): boolean {
    for (let i = 0; i < incidents.length; i++) {
      if (!this._checked.has(incidents[i].key)) {
        return false;
      }
    }
    return true;
  }

  incidentListForBot(incidents: Incident[]): TemplateResult {
    const incidentsHTML = incidents.map((i) => html`<li @click=${() => this._select(i)}>${i.params.alertname}</li>`);
    return html`<ul class=bot-incident-elem>${incidentsHTML}</ul>`;
  }

  incidentList(incidents: Incident[], isBotCentricView: boolean): TemplateResult[] {
    if (isBotCentricView) {
      return this.botCentricView();
    }
    return incidents.map((i) => html`
        <h2 class=${this.classOfH2(i)} @click=${() => this._select(i)}>
        <span class=noselect>
          <checkbox-sk ?checked=${this._checked.has(i.key)} @change=${this._check_selected} @click=${this._clickHandler} id=${i.key}></checkbox-sk>
          ${this.assignedTo(i)}
          ${this.displayIncident(i)}
        </span>
        <span>
          <alarm-off-icon-sk title='This incident has a recently expired silence' class=${this.hasRecentlyExpiredSilence(i)}></alarm-off-icon-sk>
          <comment-icon-sk title='This incident has notes.' class=${this.hasNotes(i)}></comment-icon-sk>
        </span>
        </h2>
      `);
  }

  statsList(): TemplateResult[] {
    return this._stats.map((stat: Stat) => html`<h2 @click=${() => this._statsClick(stat.incident)}>${this.displayIncident(stat.incident)} <span>${stat.num}</span></h2>`);
  }

  numMatchSilence(s: Silence): number {
    if (!this._incidents) {
      return 0;
    }
    return this._incidents.filter(
      (incident: Incident) => paramset.match(s.param_set, incident.params) && incident.active,
    ).length;
  }

  clearSelections(): TemplateResult {
    return html`<button class=selection ?disabled=${this._checked.size === 0} @click=${this._clearSelections}>Clear selections</button>`;
  }

  assignMultiple(): TemplateResult {
    return html`<button class=selection ?disabled=${this._checked.size === 0} @click=${this._assignMultiple}>Assign ${this._checked.size} alerts</button>`;
  }

  botCentricBtn(): TemplateResult {
    let buttonText;
    if (this._isBotCentricView) {
      buttonText = 'Switch to Normal view';
    } else {
      buttonText = 'Switch to Bot-centric view';
    }
    return html`<button @click=${this._flipBotCentricView}>${buttonText}</button>`;
  }

  findParent(ele: HTMLElement|null, tagName: string): HTMLElement|null {
    while (ele && (ele.tagName !== tagName)) {
      ele = ele.parentElement;
    }
    return ele;
  }

  _poll(stopSpinner: boolean): void {
    const incidents = fetch('/_/incidents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json: IncidentsResponse) => {
      this._incidents = json.incidents || [];
      // If alert_id is specified and it is in supported rhs_states then display
      // an incident.
      if ((this._rhs_state === START || this._rhs_state === INCIDENT)
          && this._state.alert_id) {
        for (let i = 0; i < this._incidents.length; i++) {
          if (this._incidents[i].id === this._state.alert_id) {
            this._select(this._incidents[i]);
            break;
          }
        }
      }
      this._incidents = json.incidents || [];
      this._incidentsToRecentlyExpired = json.ids_to_recently_expired_silences;
    });

    const silences = fetch('/_/silences', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json: Silence[]) => {
      this._silences = json;
    });

    const emails = fetch('/_/emails', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json: string[]) => {
      this._emails = json;
    });

    Promise.all([incidents, silences, emails]).then(() => { this._render(); }).catch((msg) => {
      if (msg.resp) {
        msg.resp.text().then(errorMessage);
      } else {
        errorMessage(msg);
      }
    }).finally(() => {
      if (stopSpinner) {
        this.spinner!.active = false;
      }
      window.setTimeout(() => this._poll(false), 10000);
    });
  }

  _tabSwitch(e: CustomEvent): void {
    this._state.tab = e.detail.index;
    // Unset alert_id when switching tabs.
    this._state.alert_id = '';
    this.stateHasChanged();

    // If tab is stats then load stats.
    if (e.detail.index === 3) {
      this._getStats();
    }
    // If tab is silences then display empty silence to populate from scratch.
    // This will go away if any existing silence is clicked on.
    if (e.detail.index === 2) {
      fetch('/_/new_silence', {
        credentials: 'include',
      }).then(jsonOrThrow).then((json: Silence) => {
        this._selected = null;
        this._current_silence = json;
        this._rhs_state = EDIT_SILENCE;
        this._render();
      }).catch(errorMessage);
    } else {
      this._rhs_state = START;
      this._render();
    }
  }

  _clickHandler(e: KeyboardEvent): void {
    this._shift_pressed_during_click = e.shiftKey;
    e.stopPropagation();
  }

  _silenceClick(silence: Silence): void {
    this._current_silence = JSON.parse(JSON.stringify(silence));
    this._selected = silence;
    this._rhs_state = EDIT_SILENCE;
    this._render();
  }

  _statsClick(incident: Incident): void {
    this._selected = incident;
    this._incidentStats();
    this._rhs_state = VIEW_STATS;
  }

  // Update the paramset for a silence as Incidents are checked and unchecked.
  // TODO(jcgregorio) Remove this once checkbox-sk is fixed.
  _check_selected_impl(key: string, isChecked: boolean): void {
    if (isChecked) {
      this._last_checked_incident = key;
      this._checked.add(key);
      this._incidents.forEach((i) => {
        if (i.key === key) {
          paramset.add(this._current_silence!.param_set, i.params, this._ignored);
        }
      });
    } else {
      this._last_checked_incident = null;
      this._checked.delete(key);
      this._current_silence!.param_set = {};
      this._incidents.forEach((i) => {
        if (this._checked.has(i.key)) {
          paramset.add(this._current_silence!.param_set, i.params, this._ignored);
        }
      });
    }

    if (this._isBotCentricView) {
      this._make_bot_centric_param_set(this._current_silence!.param_set);
    }
    this._rhs_state = EDIT_SILENCE;
    this._render();
  }

  // Goes through the paramset and leaves only silence keys that are useful
  // in bot-centric view like 'alertname' and 'bot'.
  _make_bot_centric_param_set(target_paramset: ParamSet): void {
    Object.keys(target_paramset).forEach((key) => {
      if (BOT_CENTRIC_PARAMS.indexOf(key) === -1) {
        delete target_paramset[key];
      }
    });
  }

  _check_selected(e: Event): void {
    const checkbox = this.findParent(e.target as HTMLElement, 'CHECKBOX-SK') as CheckOrRadio;
    const incidents_to_check: string[] = [];
    if (this._isBotCentricView && this._bots_to_incidents
        && this._bots_to_incidents[checkbox.id]) {
      this._bots_to_incidents[checkbox.id].forEach((i) => {
        incidents_to_check.push(i.key);
      });
    } else {
      incidents_to_check.push(checkbox.id);
    }
    const checkSelectedImplFunc = () => {
      incidents_to_check.forEach((id) => {
        this._check_selected_impl(id, checkbox.checked);
      });
    };

    if (!this._checked.size) {
      // Request a new silence.
      fetch('/_/new_silence', {
        credentials: 'include',
      }).then(jsonOrThrow).then((json) => {
        this._selected = null;
        this._current_silence = json;
        checkSelectedImplFunc();
      }).catch(errorMessage);
    } else if (this._shift_pressed_during_click && this._last_checked_incident) {
      let foundStart = false;
      let foundEnd = false;
      // Find all incidents included in the range during shift click.
      const incidents_included_in_range: string[] = [];

      // The incidents we go through for shift click selections will be
      // different for bot-centric vs normal view.
      const incidents = this._isBotCentricView
        ? ([] as Incident[]).concat(...Object.values(this._bots_to_incidents))
        : this._incidents;

      incidents.some((i) => {
        if (i.key === this._last_checked_incident
                || incidents_to_check.includes(i.key)) {
          if (!foundStart) {
            // This is the 1st time we have entered this block. This means we
            // found the first incident.
            foundStart = true;
          } else {
            // This is the 2nd time we have entered this block. This means we
            // found the last incident.
            foundEnd = true;
          }
        }
        if (foundStart) {
          incidents_included_in_range.push(i.key);
        }
        return foundEnd;
      });

      if (foundStart && foundEnd) {
        incidents_included_in_range.forEach((key) => {
          this._check_selected_impl(key, true);
        });
      } else {
        // Could not find start and/or end incident. Only check the last
        // clicked.
        checkSelectedImplFunc();
      }
    } else {
      checkSelectedImplFunc();
    }
  }

  _select(incident: Incident): void {
    this._state.alert_id = incident.id;
    this.stateHasChanged();

    this._rhs_state = INCIDENT;
    this._checked = new Set();
    this._selected = incident;
    this._current_silence = null;
    this._render();
  }

  _addNote(e: CustomEvent): void {
    this._doImpl('/_/add_note', e.detail);
  }

  _delNote(e: CustomEvent): void {
    this._doImpl('/_/del_note', e.detail);
  }

  _addSilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you add a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _deleteSilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you delete a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _modifySilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you modify a param.
    if (!silence.key) {
      this._current_silence = silence;
      this._render();
      return;
    }
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, false));
  }

  _saveSilence(silence: Silence): void {
    this._checked = new Set();
    this._doImpl('/_/save_silence', silence, (json) => this._silenceAction(json, true));
  }

  _archiveSilence(silence: Silence): void {
    this._doImpl('/_/archive_silence', silence, (json) => this._silenceAction(json, true));
  }

  _reactivateSilence(silence: Silence): void {
    this._doImpl('/_/reactivate_silence', silence, (json) => this._silenceAction(json, false));
  }

  _deleteSilence(silence: Silence): void {
    this._doImpl('/_/del_silence', silence, (json) => {
      for (let i = 0; i < this._silences.length; i++) {
        if (this._silences[i].key === json.key) {
          this._silences.splice(i, 1);
          this._rhs_state = START;
          break;
        }
      }
    });
  }

  _addSilenceNote(e: CustomEvent): void {
    this._doImpl('/_/add_silence_note', e.detail, (json) => this._silenceAction(json, false));
  }

  _delSilenceNote(e: CustomEvent): void {
    this._doImpl('/_/del_silence_note', e.detail, (json) => this._silenceAction(json, false));
  }

  _botChooser(): void {
    this.populateBotsToIncidents(this._incidents);
    ($$('#bot-chooser', this) as BotChooserSk).open(this._bots_to_incidents, this._current_silence!.param_set.bot!).then((bot) => {
      if (!bot) {
        return;
      }
      const bot_incidents = this._bots_to_incidents[bot];
      bot_incidents.forEach((i) => {
        const bot_centric_params: Params = {};
        BOT_CENTRIC_PARAMS.forEach((p) => {
          bot_centric_params[p] = i.params[p];
        });
        paramset.add(this._current_silence!.param_set, bot_centric_params, this._ignored);
      });
      this._modifySilenceParam(this._current_silence!);
    });
  }

  _assign(e: CustomEvent): void {
    const owner = this._selected && (this._selected as Incident).params.owner;
    ($$('#email-chooser', this) as EmailChooserSk).open(this._emails, owner!).then((email) => {
      const detail = {
        key: e.detail.key,
        email: email,
      };
      this._doImpl('/_/assign', detail);
    });
  }

  _flipBotCentricView(): void {
    this._isBotCentricView = !this._isBotCentricView;
    this._render();
  }

  _assignMultiple(): void {
    const owner = (this._selected && (this._selected as Incident).params.owner) || '';
    ($$('#email-chooser', this) as EmailChooserSk).open(this._emails, owner).then((email) => {
      const detail = {
        keys: Array.from(this._checked),
        email: email,
      };
      this._doImpl('/_/assign_multiple', detail, (json) => {
        this._incidents = json;
        this._checked = new Set();
        this._render();
      });
    });
  }

  _clearSelections(): void {
    this._checked = new Set();
    this._render();
  }

  _assignToOwner(e: CustomEvent): void {
    const owner = this._selected && (this._selected as Incident).params.owner;
    const detail = {
      key: e.detail.key,
      email: owner,
    };
    this._doImpl('/_/assign', detail);
  }

  _take(e: CustomEvent): void {
    this._doImpl('/_/take', e.detail);
    // Do not do desktop notification on takes, it is redundant.
    this._incidents_notified[e.detail.key] = true;
  }

  _getStats(): void {
    const detail: StatsRequest = {
      range: this._stats_range,
    };
    this._doImpl('/_/stats', detail, (json: Stat[]) => this._statsAction(json));
  }

  _incidentStats(): void {
    const detail: IncidentsInRangeRequest = {
      incident: this._selected as Incident,
      range: this._stats_range,
    };
    this._doImpl('/_/incidents_in_range', detail, (json: Incident[]) => this._incidentStatsAction(json));
  }

  // Actions to take after updating incident stats.
  _incidentStatsAction(json: Incident[]): void {
    this._incident_stats = json;
  }

  // Actions to take after updating Stats.
  _statsAction(json: Stat[]): void {
    this._stats = json;
  }

  // Actions to take after updating an Incident.
  _incidentAction(json: Incident): void {
    const incidents = this._incidents;
    for (let i = 0; i < incidents.length; i++) {
      if (incidents[i].key === json.key) {
        incidents[i] = json;
        break;
      }
    }
    this._selected = json;
  }

  // Actions to take after updating a Silence.
  _silenceAction(json: Silence, clear: boolean): void {
    let found = false;
    this._current_silence = json;
    for (let i = 0; i < this._silences.length; i++) {
      if (this._silences[i].key === json.key) {
        this._silences[i] = json;
        found = true;
        break;
      }
    }
    if (!found) {
      this._silences.push(json);
    }
    if (clear) {
      this._rhs_state = START;
    }
  }

  // Common work done for all fetch requests.
  _doImpl(url: string, detail: any, action = (json: any) => this._incidentAction(json)): void {
    this.spinner!.active = true;
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
      this.spinner!.active = false;
    }).catch((msg) => {
      this.spinner!.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  // Fix-up all the incidents and silences, including re-sorting them.
  _rationalize(): void {
    this._incidents.forEach((incident) => {
      const silenced = this._silences.reduce((isSilenced, silence) => isSilenced
              || (silence.active && paramset.match(silence.param_set, incident.params)), false);
      incident.params.__silence_state = silenced ? 'silenced' : 'active';
    });

    // Sort the incidents, using the following 'sortby' list as tiebreakers.
    const sortby = ['__silence_state', 'assigned_to', 'alertname', 'abbr', 'id'];
    this._incidents.sort((a, b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      // Inactive incidents are then sorted by 'lastseen' timestamp.
      if (!a.active) {
        const delta = b.last_seen - a.last_seen;
        if (delta) {
          return delta;
        }
      }
      for (let i = 0; i < sortby.length; i++) {
        const key = sortby[i];
        const left = a.params[key] || '';
        const right = b.params[key] || '';
        const cmp = left.localeCompare(right);
        if (cmp) {
          return cmp;
        }
      }
      return 0;
    });
    this._silences.sort((a, b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      return b.updated - a.updated;
    });
  }

  _needsTriaging(incident: Incident, isInfraGardener: boolean): boolean {
    if (incident.active
      && (incident.params.__silence_state !== 'silenced')
      && (
        (isInfraGardener && !incident.params.assigned_to)
        || (incident.params.assigned_to === this._user)
        || (incident.params.owner === this._user
            && !incident.params.assigned_to)
      )
    ) {
      return true;
    }
    return false;
  }

  _requestDesktopNotificationPermission(): void {
    if (Notification && Notification.permission === 'default') {
      Notification.requestPermission();
    }
  }

  _sendDesktopNotification(unNotifiedIncidents: Incident[]): void {
    if (unNotifiedIncidents.length === 0) {
      // Do nothing.
      return;
    }
    let text = '';
    if (unNotifiedIncidents.length === 1) {
      text = `${unNotifiedIncidents[0].params.alertname}\n\n${unNotifiedIncidents[0].params.description}`;
    } else {
      text = `There are ${unNotifiedIncidents.length} alerts assigned to you`;
    }
    const notification = new Notification('am.skia.org notification', {
      icon: '/static/icon-active.png',
      body: text,
      // 'tag' handles multi-tab scenarios. When multiple tabs are open then
      // only one notification is sent for the same alert.
      tag: `alertManagerNotification${text}`,
    });
    // onclick move focus to the am.skia.org tab and close the notification.
    notification.onclick = () => {
      window.parent.focus();
      window.focus(); // Supports older browsers.
      this._select(unNotifiedIncidents[0]); // Display the 1st incident.
      notification.close();
    };
    setTimeout(notification.close.bind(notification), 10000);
  }

  _render(): void {
    this._rationalize();
    render(AlertManagerSk.template(this), this, { eventContext: this });
    // Update the icon.
    const isInfraGardener = this._user === this._infra_gardener;
    const numActive = this._incidents.reduce((n, incident) => n += this._needsTriaging(incident, isInfraGardener) ? 1 : 0, 0);

    // Show desktop notifications only if permission was granted and only if
    // silences have been successfully fetched. If silences have not been
    // fetched yet then we might end up notifying on silenced incidents.
    if (Notification.permission === 'granted' && this._silences.length !== 0) {
      const unNotifiedIncidents = this._incidents.filter((i) => !this._incidents_notified[i.key] && this._needsTriaging(i, isInfraGardener));
      this._sendDesktopNotification(unNotifiedIncidents);
      unNotifiedIncidents.forEach((i) => this._incidents_notified[i.key] = true);
    }

    document.title = `${numActive} - AlertManager`;
    if (!this._favicon) {
      return;
    }
    if (numActive > 0) {
      this._favicon.href = '/static/icon-active.png';
    } else {
      this._favicon.href = '/static/icon.png';
    }
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  // private _stateHasChanged: ()=> void = () => {};
}

define('alert-manager-sk', AlertManagerSk);

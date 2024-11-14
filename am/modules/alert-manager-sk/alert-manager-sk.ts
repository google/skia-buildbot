/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */
import { html, render, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/alarm-off-icon-sk';
import '../../../elements-sk/modules/icons/comment-icon-sk';
import '../../../elements-sk/modules/icons/notifications-icon-sk';
import '../../../elements-sk/modules/icons/person-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/tabs-panel-sk';
import '../../../elements-sk/modules/tabs-sk';
import '../../../elements-sk/modules/toast-sk';

import '../incident-sk';
import '../auto-assign-sk';
import '../bot-chooser-sk';
import '../email-chooser-sk';
import '../silence-sk';

import { diffDate } from '../../../infra-sk/modules/human';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { AutoAssignSk } from '../auto-assign-sk/auto-assign-sk';
import { BotChooserSk } from '../bot-chooser-sk/bot-chooser-sk';
import { EmailChooserSk } from '../email-chooser-sk/email-chooser-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

import * as paramset from '../paramset';
import { displaySilence, expiresIn, getSilenceFullName } from '../am';

import {
  Silence,
  Incident,
  StatsRequest,
  Stat,
  IncidentsResponse,
  ParamSet,
  Params,
  IncidentsInRangeRequest,
  AuditLog,
} from '../json';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';

// Legal states.
const START = 'start';
const INCIDENT = 'incident';
const EDIT_SILENCE = 'edit_silence';
const VIEW_STATS = 'view_stats';
const VIEW_AUDITLOG = 'view_auditlog';

const MAX_SILENCES_TO_DISPLAY_IN_TAB = 50;

const BOT_CENTRIC_PARAMS = ['alertname', 'bot'];

class State {
  tab: number = 0; // The selected tab.

  alert_id: string = ''; // The selected alert (if any).
}

// This response structure comes from chrome-ops-rotation-proxy.appspot.com.
// We do not have access to the structure to generate TS.
interface RotationResp {
  emails: string[];
}

export class AlertManagerSk extends HTMLElement {
  private incidents: Incident[] = []; // All active incidents.

  private filterSilencesVal: string = '';

  private filterAuditLogsVal: string = '';

  private silences: Silence[] = []; // All active silences.

  private stats: Stat[] = []; // Last requested stats.

  private audit_logs: AuditLog[] = [];

  private stats_range = '1w';

  private incident_stats: Incident[] = []; // The incidents for a given stat.

  private rhs_state = START; // One of START, INCIDENT, or EDIT_SILENCE.

  private selected: Incident | Silence | null = null; // The selected incident, i.e. you clicked on the name.

  private checked = new Set(); // Checked incidents, i.e. you clicked the checkbox.

  private bots_to_incidents: Record<string, Incident[]> = {}; // Bot names to their incidents. Used in bot-centric view.

  private isBotCentricView = false; // Determines if bot-centric view is displayed on incidents tab.

  private current_silence: Silence | null = null; // A silence under construction.

  // Params to ignore when constructing silences.
  private ignored = [
    '__silence_state',
    'description',
    'id',
    'swarming',
    'assigned_to',
    'kubernetes_pod_name',
    'instance',
    'pod_template_hash',
    'abbr_owner_regex',
    'controller_revision_hash',
  ];

  private shift_pressed_during_click = false; // If the shift key was held down during the mouse click.

  private last_checked_incident: string | null = null; // Keeps track of the last checked incident. Used for multi-selecting incidents with shift.

  private incidents_notified: Record<string, boolean> = {}; // Keeps track of all incidents that were notified via desktop notifications.

  private incidentsToRecentlyExpired: Record<string, boolean> = {}; // Map of incident IDs to whether their silences recently expired.

  private user = 'barney@example.org';

  private infra_gardener = '';

  private skia_gardener = '';

  // State is reflected to the URL via stateReflector.
  private state: State = {
    tab: 0,
    alert_id: '',
  };

  private favicon: HTMLAnchorElement | null = null;

  private spinner: SpinnerSk | null = null;

  private emails: string[] = [];

  private helpDialog: HTMLDialogElement | null = null;

  // When enabled, text in the sidebar will be selectable, and alert titles will be displayed in
  // full length. Useful for copying and pasting alerts into the Skia Infra Gardener Handoff doc.
  private inSelectionMode = false;

  constructor() {
    super();

    fetch('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener', {
      mode: 'cors',
    })
      .then(jsonOrThrow)
      .then((json: RotationResp) => {
        this.skia_gardener = json.emails[0];
        this._render();
      });
    fetch('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:browser-infra-tooling', {
      mode: 'cors',
    })
      .then(jsonOrThrow)
      .then((json: RotationResp) => {
        this.infra_gardener = json.emails[0];
        this._render();
      });
    LoggedIn().then((loginstatus) => {
      this.user = loginstatus.email;
      this._render();
    });
  }

  private static template = (ele: AlertManagerSk) => html`
<header>
  ${ele.gardener()}
  <theme-chooser-sk></theme-chooser-sk>
</header>
<dialog id=help>
  <h2>Keyboard Controls</h2>
  <table>
    <tr><td class=mono>'ArrowDown'</td><td>Move down the list of incidents.</td></tr>
    <tr><td class=mono>'ArrowUp'</td><td>Move up the list of incidents.</td></tr>
    <tr><td class=mono>'Space'</td><td>Check the incident to create a silence or to assign.</td></tr>
    <tr><td class=mono>'Shift+ArrowDown'</td><td>Check the below incident to create a silence (only if the current incident is checked).</td></tr>
    <tr><td class=mono>'Shift+ArrowUp'</td><td>Check the above incident to create a silence (only if the current incident is checked).</td></tr>
    <tr><td class=mono>'ArrowRight'</td><td>Move to the first textarea in RHS.</td></tr>
    <tr><td class=mono>'a'</td><td>Assign selected alert(s).</td></tr>
    <tr><td class=mono>'A'</td><td>Bring up the auto-assign dialog.</td></tr>
    <tr><td class=mono>'b'</td><td>Switches view from normal to bot-centric view.</td></tr>
    <tr><td class=mono>'1'</td><td>Switches to the "Mine" tab.</td></tr>
    <tr><td class=mono>'2'</td><td>Switches to the "Infra Alerts" tab.</td></tr>
    <tr><td class=mono>'3'</td><td>Switches to the "Skia Alerts" tab.</td></tr>
    <tr><td class=mono>'4'</td><td>Switches to the "All Alerts" tab.</td></tr>
    <tr><td class=mono>'?'</td><td>Show help.</td></tr>
    <tr><td class=mono>'Esc'</td><td>Stop showing help.</td></tr>
  </table>
  <div class=footnote>
    Note: Keyboard controls only work in the 'Mine', 'Infra Alerts', 'Skia Alerts', and 'All Alerts' tabs. 'Bot-centric view' is also not supported.
  </div>
</dialog>
<section class=nav>
  <tabs-sk @tab-selected-sk=${ele.tabSwitch} selected=${ele.state.tab}>
    <button class="tab">Mine</button>
    <button class="tab">Infra</button>
    <button class="tab">Skia</button>
    <button class="tab">All</button>
    <button class="tab">Silences</button>
    <button class="tab">Stats</button>
    <button class="tab">Audit</button>
  </tabs-sk>
  <tabs-panel-sk>
    <section class=mine>
      <span class=selection-buttons>
        ${ele.displayAssignMultiple()}
        ${ele.displayClearSelections()}
      </span>
      ${ele.incidentList(ele.getMyIncidents(), false)}
    </section>
    <section class=infra-incidents>
      <span class=selection-buttons>
        ${ele.displayAssignMultiple()}
        ${ele.displayClearSelections()}
        ${ele.displayAutoAssign()}
      </span>
      ${ele.incidentList(ele.getInfraIncidents(), ele.isBotCentricView)}
    </section>
    <section class=skia-incidents>
      ${ele.botCentricBtn()}
      <span class=selection-buttons>
        ${ele.displayAssignMultiple()}
        ${ele.displayClearSelections()}
        ${ele.displayAutoAssign()}
      </span>
      ${ele.incidentList(ele.getSkiaIncidents(), ele.isBotCentricView)}
    </section>
    <section class=incidents>
      ${ele.botCentricBtn()}
      <span class=selection-buttons>
        ${ele.displayAssignMultiple()}
        ${ele.displayClearSelections()}
        ${ele.displayAutoAssign()}
      </span>
      ${ele.incidentList(ele.incidents, ele.isBotCentricView)}
    </section>
    <section class=silences>
      <input class=silences-filter placeholder="Filter silences" .value="${
        ele.filterSilencesVal
      }" @input=${(e: Event) => ele.filterSilencesEvent(e)}></input>
      <br/><br/>
      ${ele.silences
        .filter((silence: Silence) =>
          getSilenceFullName(silence.param_set).includes(ele.filterSilencesVal)
        )
        .slice(0, MAX_SILENCES_TO_DISPLAY_IN_TAB)
        .map(
          (i: Silence) =>
            html` <h2 class=${ele.classOfSilenceH2(i)} @click=${() => ele.silenceClick(i)}>
              <span> ${displaySilence(i.param_set)} </span>
              <span>
                <span title="Expires in">${expiresIn(i.active, i.created, i.duration)}</span>
                <comment-icon-sk
                  title="This silence has notes."
                  class=${ele.hasNotes(i)}></comment-icon-sk>
                <span title="The number of active alerts that match this silence."
                  >${ele.numMatchSilence(i)}</span
                >
              </span>
            </h2>`
        )}
    </section>
    <section class=stats>
      ${ele.statsList()}
    </section>
    <section class=auditlogs>
      <input class=auditlogs-filter placeholder="Filter audit logs" .value="${
        ele.filterAuditLogsVal
      }" @input=${(e: Event) => ele.filterAuditLogsEvent(e)}></input>
    </section>
  </tabs-panel-sk>
  <checkbox-sk
    class="selection-mode-toggle"
    ?checked=${ele.inSelectionMode}
    @change=${ele.selectionModeChange}
    label="Selection mode"
    title="${
      'Allows selecting text in the sidebar. Useful for copying and pasting into the ' +
      'Skia Gardener Handoff Doc.'
    }"
  ></checkbox-sk>
</section>
<section class=edit>
  ${ele.rightHandSide()}
</section>
<footer>
  <spinner-sk id=busy></spinner-sk>
  <auto-assign-sk id=auto-assign></auto-assign-sk>
  <bot-chooser-sk id=bot-chooser></bot-chooser-sk>
  <email-chooser-sk id=email-chooser></email-chooser-sk>
  <error-toast-sk></error-toast-sk>
</footer>
`;

  connectedCallback(): void {
    this.requestDesktopNotificationPermission();

    this.addEventListener('save-silence', (e) =>
      this.saveSilence((e as CustomEvent).detail.silence)
    );
    this.addEventListener('archive-silence', (e) =>
      this.archiveSilence((e as CustomEvent).detail.silence)
    );
    this.addEventListener('reactivate-silence', (e) =>
      this.reactivateSilence((e as CustomEvent).detail.silence)
    );
    this.addEventListener('delete-silence', (e) =>
      this.deleteSilence((e as CustomEvent).detail.silence)
    );
    this.addEventListener('add-silence-note', (e) => this.addSilenceNote(e as CustomEvent));
    this.addEventListener('del-silence-note', (e) => this.delSilenceNote(e as CustomEvent));
    this.addEventListener('add-silence-param', (e) =>
      this.addSilenceParam((e as CustomEvent).detail.silence)
    );
    this.addEventListener('delete-silence-param', (e) =>
      this.deleteSilenceParam((e as CustomEvent).detail.silence)
    );
    this.addEventListener('modify-silence-param', (e) =>
      this.modifySilenceParam((e as CustomEvent).detail.silence)
    );
    this.addEventListener('add-note', (e) => this.addNote(e as CustomEvent));
    this.addEventListener('del-note', (e) => this.delNote(e as CustomEvent));
    this.addEventListener('take', (e) => this.take(e as CustomEvent));
    this.addEventListener('bot-chooser', () => this.botChooser());
    this.addEventListener('assign', (e) => this.assign(e as CustomEvent));
    this.addEventListener('assign-to-owner', (e) => this.assignToOwner(e as CustomEvent));
    // For keyboard navigation.
    document.addEventListener('keydown', (e) => this.keyDown(e));

    this.stateHasChanged = stateReflector(
      /* getState */ () => this.state as unknown as HintableObject,
      /* setState */ (newState) => {
        this.state = newState as unknown as State;
        // Set rhs_side to AUDIT_LOG if tab is on Audit.
        if (this.state.tab === 4) {
          this.getAuditLogs();
          this.rhs_state = VIEW_AUDITLOG;
        }
        this._render();
      }
    );

    this._render();

    this.helpDialog = $$('#help', this);
    this.spinner = $$('#busy', this) as SpinnerSk;
    this.favicon = $$('#favicon');

    this.spinner.active = true;
    this.poll(true);
  }

  private getMyIncidents(): Incident[] {
    return this.incidents.filter(
      (i: Incident) =>
        i.active &&
        i.params.__silence_state !== 'silenced' &&
        ((this.user === this.infra_gardener && i.params['category'] === 'infra') ||
          (this.user === this.skia_gardener && i.params['category'] === 'skia') ||
          i.params.assigned_to === this.user ||
          (i.params.owner === this.user && !i.params.assigned_to))
    );
  }

  private getInfraIncidents(): Incident[] {
    return this.incidents.filter((i: Incident) => i.params['category'] === 'infra');
  }

  private getSkiaIncidents(): Incident[] {
    return this.incidents.filter((i: Incident) => i.params['category'] === 'skia');
  }

  private keyDown(e: KeyboardEvent) {
    // Ignore all tabs other than the mine and incident tabs.
    if (this.state.tab !== 0 && this.state.tab !== 1) {
      return;
    }

    // Too complicated to navigate through issues in bot centric view
    // because the layout of incidents is completely different than
    // the standard view.
    if (this.isBotCentricView) {
      return;
    }

    // Ignore IME composition events.
    if (e.isComposing || e.keyCode === 229) {
      return;
    }

    // Do not perform keyboard navigation if input/textarea/select are
    // selected.
    const focusedElem: Element | null = document.activeElement;
    const ignoreKeyboardNavigationInTags = ['input', 'textarea', 'select'];
    if (focusedElem && ignoreKeyboardNavigationInTags.includes(focusedElem.tagName.toLowerCase())) {
      return;
    }

    switch (e.key) {
      case '?':
        (this.helpDialog! as any).showModal();
        break;
      case 'ArrowUp':
        this.keyboardNavigateIncidents(e, true);
        break;
      case 'ArrowDown':
        this.keyboardNavigateIncidents(e, false);
        break;
      case 'ArrowRight':
        // Put focus on the 1st textarea on the page (if one exists).
        if ($('textarea') && $('textarea').length > 0) {
          ($('textarea')[0] as HTMLElement).focus();
        }
        break;
      case ' ':
        if (this.selected) {
          ($$(`#${this.selected!.key}`)! as HTMLElement).click();
        } else if (this.checked.size > 0) {
          // No incident was selected. Clear all the checked incidents and
          // start from the first incident.
          this.checked.clear();
          this.last_checked_incident = null;
          if (this.incidents.length > 0) {
            this.selected = this.incidents[0];
          }
          this._render();
        }
        e.preventDefault();
        break;
      case 'b':
        this.flipBotCentricView();
        break;
      case 'a':
        // If an alert is selected and nothing is checked then
        // check the selected alert before assigning it.
        if (this.selected && this.checked.size === 0) {
          ($$(`#${this.selected!.key}`)! as HTMLElement).click();
        }
        this.assignMultiple();
        break;
      case 'A':
        this.autoAssign();
        break;
      case '1':
        this.keyboardNavigateTabs(0);
        break;
      case '2':
        this.keyboardNavigateTabs(1);
        break;
      default:
        break;
    }
  }

  private keyboardNavigateTabs(tabIndex: number) {
    this.state.tab = tabIndex;
    this.state.alert_id = '';
    this.selected = null;
    this.checked.clear();
    this.rhs_state = START;
    this.stateHasChanged();
    this._render();
  }

  private keyboardNavigateIncidents(e: KeyboardEvent, reverseDirection: boolean) {
    // If an alert is not selected or checked then the first incident in the
    // incidents array can be starting point.
    let foundStartingPoint = this.selected === null && this.checked.size === 0;

    // Figure out which incidents we are looking at.
    let incidentsArray = this.incidents;
    if (this.state.tab === 0) {
      // Use "my" incidents if we are on the "Mine" tab.
      incidentsArray = this.getMyIncidents();
    }
    if (reverseDirection && !foundStartingPoint) {
      // We only want to reverse incidents if we have not yet
      // found a starting point else we will start with the
      // last alert.
      incidentsArray = incidentsArray.reverse();
    }

    for (const incident of incidentsArray) {
      if (foundStartingPoint) {
        const k = incident.key;
        // If shift key is pressed then check the incident if it is not already
        // checked.
        if (e.shiftKey && this.last_checked_incident) {
          if (this.checked.has(k)) {
            // This incident is already checked. Set it as the
            // last_checked_incident so we can proceed from here the next time.
            this.last_checked_incident = k;
          } else {
            // Check the incident.
            ($$(`#${k}`)! as HTMLElement).click();
          }
        } else {
          ($$(`#container-${k}`)! as HTMLElement).click();
        }
        break;
      }

      if (this.selected && this.selected.key === incident.key) {
        // We found the selected incident, the next incident in the iteration
        // is the one we need to process.
        foundStartingPoint = true;
      } else if (this.checked.size > 0 && this.last_checked_incident === incident.key) {
        // We found the last checked incident, the next incident in the
        // iteration is the one we need to process.
        foundStartingPoint = true;
      }
    }
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private classOfH2(incident: Incident): string {
    const ret = [];
    if (!incident.active) {
      ret.push('inactive');
    } else if (incident.params.__silence_state === 'silenced') {
      ret.push('silenced');
    } else if (incident.params.assigned_to) {
      ret.push('assigned');
    }
    if (this.selected && this.selected.key === incident.key) {
      ret.push('selected');
    }
    return ret.join(' ');
  }

  private classOfSilenceH2(silence: Silence): string {
    const ret = [];
    if (!silence.active) {
      ret.push('inactive');
    }
    if (this.selected && this.selected.key === silence.key) {
      ret.push('selected');
    }
    return ret.join(' ');
  }

  private editIncident(): TemplateResult {
    if (this.selected) {
      return html`<incident-sk
        .incident_silences=${this.silences}
        .incident_state=${this.selected}></incident-sk>`;
    }
    return html``;
  }

  private editSilence(): TemplateResult {
    return html`<silence-sk
      .silence_state=${this.current_silence}
      .silence_incidents=${this.incidents}></silence-sk>`;
  }

  private viewStats(): TemplateResult[] {
    return this.incident_stats.map(
      (i, index) =>
        html`<incident-sk .incident_state=${i} minimized params=${index === 0}></incident-sk>`
    );
  }

  private rightHandSide(): TemplateResult | TemplateResult[] {
    switch (this.rhs_state) {
      case START:
        return [];
      case INCIDENT:
        return this.editIncident();
      case EDIT_SILENCE:
        return this.editSilence();
      case VIEW_STATS:
        return this.viewStats();
      case VIEW_AUDITLOG:
        return this.viewAuditLogsTable();
      default:
        return [];
    }
  }

  private hasNotes(o: Incident | Silence): string {
    return o.notes && o.notes.length > 0 ? '' : 'invisible';
  }

  private hasRecentlyExpiredSilence(incident: Incident): string {
    return this.incidentsToRecentlyExpired[incident.id] ? '' : 'invisible';
  }

  private displayIncident(incident: Incident): TemplateResult {
    const ret = [incident.params.alertname];
    const abbr = incident.params.abbr;
    if (abbr) {
      ret.push(` - ${abbr}`);
    }
    const fullIncident = ret.join(' ');
    let displayIncident = fullIncident;
    if (!this.inSelectionMode && displayIncident.length > 48) {
      displayIncident = `${displayIncident.slice(0, 45)}...`;
    }
    return html`<span title="${fullIncident}">${displayIncident}</span>`;
  }

  private gardener(): TemplateResult {
    if (this.infra_gardener === this.user) {
      return html`<notifications-icon-sk
        title="You are the Infra Gardener, awesome!"></notifications-icon-sk>`;
    }
    if (this.skia_gardener === this.user) {
      return html`<notifications-icon-sk
        title="You are the Skia Gardener, awesome!"></notifications-icon-sk>`;
    }
    return html``;
  }

  private assignedTo(incident: Incident): TemplateResult {
    if (incident.params.assigned_to === this.user) {
      return html`<person-icon-sk title="This item is assigned to you."></person-icon-sk>`;
    }
    if (incident.params.assigned_to) {
      return html`<span
        class="assigned-circle"
        title="This item is assigned to ${incident.params.assigned_to}."
        >${incident.params.assigned_to[0].toUpperCase()}</span
      >`;
    }
    return html``;
  }

  private populateBotsToIncidents(incidents: Incident[]): void {
    // Reset bots_to_incidents and populate it from scratch.
    this.bots_to_incidents = {};
    for (let i = 0; i < incidents.length; i++) {
      const incident = incidents[i];
      if (incident.params && incident.params.bot) {
        // Only consider active bot incidents that are not assigned or silenced.
        if (
          !incident.active ||
          incident.params.__silence_state === 'silenced' ||
          incident.params.assigned_to
        ) {
          continue;
        }
        const botName = incident.params.bot;
        if (this.bots_to_incidents[botName]) {
          this.bots_to_incidents[botName].push(incident);
        } else {
          this.bots_to_incidents[botName] = [incident];
        }
      }
    }
  }

  private botCentricView(): TemplateResult[] {
    this.populateBotsToIncidents(this.incidents);
    const botsHTML: TemplateResult[] = [];
    Object.keys(this.bots_to_incidents).forEach((botName) => {
      botsHTML.push(html`
        <h2 class="bot-centric">
          <span class="${this.inSelectionMode ? 'selection-mode' : 'noselect'}">
            <checkbox-sk
              class="bot-alert-checkbox"
              ?checked=${this.isBotChecked(this.bots_to_incidents[botName])}
              @change=${this.check_selected}
              @click=${this.clickHandler}
              id=${botName}></checkbox-sk>
            <span class="bot-alert">
              ${botName}
              <span class="bot-incident-list">
                ${this.incidentListForBot(this.bots_to_incidents[botName])}
              </span>
            </span>
          </span>
        </h2>
      `);
    });
    return botsHTML;
  }

  // Checks to see if all the incidents for the bot are checked.
  private isBotChecked(incidents: Incident[]): boolean {
    for (let i = 0; i < incidents.length; i++) {
      if (!this.checked.has(incidents[i].key)) {
        return false;
      }
    }
    return true;
  }

  private incidentListForBot(incidents: Incident[]): TemplateResult {
    const incidentsHTML = incidents.map(
      (i) => html`<li @click=${() => this.select(i)}>${i.params.alertname}</li>`
    );
    return html`<ul class="bot-incident-elem">
      ${incidentsHTML}
    </ul>`;
  }

  private incidentList(incidents: Incident[], isBotCentricView: boolean): TemplateResult[] {
    if (isBotCentricView) {
      return this.botCentricView();
    }
    return incidents.map(
      (i) => html`
        <h2 class=${this.classOfH2(i)} @click=${() => this.select(i)} id="container-${i.key}">
          <span class="${this.inSelectionMode ? 'selection-mode' : 'noselect'}">
            <checkbox-sk
              ?checked=${this.checked.has(i.key)}
              @change=${this.check_selected}
              @click=${this.clickHandler}
              id=${i.key}></checkbox-sk>
            ${this.assignedTo(i)} ${this.displayIncident(i)}
          </span>
          <span>
            <alarm-off-icon-sk
              title="This incident has a recently expired silence"
              class=${this.hasRecentlyExpiredSilence(i)}></alarm-off-icon-sk>
            <comment-icon-sk
              title="This incident has notes."
              class=${this.hasNotes(i)}></comment-icon-sk>
          </span>
        </h2>
      `
    );
  }

  private statsList(): TemplateResult[] {
    return this.stats.map(
      (stat: Stat) =>
        html`<h2 @click=${() => this.statsClick(stat.incident)}>
          ${this.displayIncident(stat.incident)} <span>${stat.num}</span>
        </h2>`
    );
  }

  private viewAuditLogsTable(): TemplateResult {
    return html`
      <table id="audit-logs-table">
        ${this.getAuditLogsRows()}
      </table>
    `;
  }

  private getAuditLogsRows(): TemplateResult[] {
    const filtered_audit_logs = this.audit_logs.filter((a) =>
      a.body.includes(this.filterAuditLogsVal)
    );
    return filtered_audit_logs.map(
      (a) => html`
        <tr>
          <td>${diffDate(a.timestamp * 1000)} ago</td>
          <td>${a.action}</td>
          <td>${a.user}</td>
          <td>${a.body}</td>
        </tr>
      `
    );
  }

  private numMatchSilence(s: Silence): number {
    if (!this.incidents) {
      return 0;
    }
    return this.incidents.filter(
      (incident: Incident) => paramset.match(s.param_set, incident.params) && incident.active
    ).length;
  }

  private displayClearSelections(): TemplateResult {
    return html`<button
      class="selection"
      ?disabled=${this.checked.size === 0}
      @click=${this.clearSelections}>
      Clear selections
    </button>`;
  }

  private getAutoAssignIncidents(): Incident[] {
    const autoAssignIncidents: Incident[] = [];
    this.incidents.forEach((incident) => {
      // We only auto-assign active alerts which are not silenced, which have
      // owners defined but are not already assigned to them.
      if (
        incident.active &&
        incident.params.__silence_state === 'active' &&
        incident.params.owner &&
        !incident.params.assigned_to
      ) {
        autoAssignIncidents.push(incident);
      }
    });
    return autoAssignIncidents;
  }

  private displayAutoAssign(): TemplateResult {
    const autoAssignIncidents = this.getAutoAssignIncidents();
    return html`<button
      class="selection"
      ?disabled=${autoAssignIncidents.length === 0}
      @click=${this.autoAssign}>
      Auto-Assign
    </button>`;
  }

  private filterSilencesEvent(e: Event): void {
    this.filterSilencesVal = (e.target as HTMLInputElement).value;
    this._render();
  }

  private filterAuditLogsEvent(e: Event): void {
    this.filterAuditLogsVal = (e.target as HTMLInputElement).value;
    this._render();
  }

  private clearSelections(): void {
    this.checked = new Set();
    this._render();
  }

  private async autoAssign(): Promise<void> {
    const autoAssignIncidents = this.getAutoAssignIncidents();
    const selectedIncidentKeys = await $$<AutoAssignSk>('#auto-assign', this)!.open(
      autoAssignIncidents
    );

    if (!selectedIncidentKeys) {
      // Nothing to do if there are no incidents to auto-assign.
      return;
    }

    this.spinner!.active = true;

    // Minimize backend calls by grouping incidents by their owners.
    const ownersToIncidentKeys: Record<string, string[]> = {};
    for (let i = 0; i < this.incidents.length; i++) {
      if (selectedIncidentKeys.includes(this.incidents[i].key)) {
        if (!ownersToIncidentKeys[this.incidents[i].params.owner]) {
          ownersToIncidentKeys[this.incidents[i].params.owner] = [];
        }
        ownersToIncidentKeys[this.incidents[i].params.owner].push(this.incidents[i].key);
      }
    }

    // Gather and execute promises that assign batches of incidents to
    // their owners.
    const promises: Promise<void>[] = Object.keys(ownersToIncidentKeys).map(
      async (owner: string): Promise<void> => {
        const detail = {
          keys: ownersToIncidentKeys[owner],
          email: owner,
        };
        const response = await fetch('/_/assign_multiple', {
          body: JSON.stringify(detail),
          headers: {
            'content-type': 'application/json',
          },
          credentials: 'include',
          method: 'POST',
        });
        const json = (await jsonOrThrow(response)) as Incident[];
        this.incidents = json;
      }
    );

    try {
      await Promise.all(promises);
      this._render();
    } catch (msg: any) {
      errorMessage(msg);
    }
    this.spinner!.active = false;
  }

  private displayAssignMultiple(): TemplateResult {
    return html`<button
      class="selection"
      ?disabled=${this.checked.size === 0}
      @click=${this.assignMultiple}>
      Assign ${this.checked.size} alerts
    </button>`;
  }

  private botCentricBtn(): TemplateResult {
    let buttonText;
    if (this.isBotCentricView) {
      buttonText = 'Switch to Normal view';
    } else {
      buttonText = 'Switch to Bot-centric view';
    }
    return html`<button @click=${this.flipBotCentricView}>${buttonText}</button>`;
  }

  private findParent(ele: HTMLElement | null, tagName: string): HTMLElement | null {
    while (ele && ele.tagName !== tagName) {
      ele = ele.parentElement;
    }
    return ele;
  }

  private poll(stopSpinner: boolean): void {
    const incidents = fetch('/_/incidents', {
      credentials: 'include',
    })
      .then(jsonOrThrow)
      .then((json: IncidentsResponse) => {
        this.incidents = json.incidents || [];
        // If alert_id is specified and it is in supported rhs_states then display
        // an incident.
        if ((this.rhs_state === START || this.rhs_state === INCIDENT) && this.state.alert_id) {
          for (let i = 0; i < this.incidents.length; i++) {
            if (this.incidents[i].id === this.state.alert_id) {
              this.select(this.incidents[i]);
              break;
            }
          }
        }
        this.incidents = json.incidents || [];
        this.incidentsToRecentlyExpired = json.ids_to_recently_expired_silences || {};
      });

    const silences = fetch('/_/silences', {
      credentials: 'include',
    })
      .then(jsonOrThrow)
      .then((json: Silence[]) => {
        this.silences = json;
      });

    const emails = fetch('/_/emails', {
      credentials: 'include',
    })
      .then(jsonOrThrow)
      .then((json: string[]) => {
        this.emails = json;
      });

    Promise.all([incidents, silences, emails])
      .then(() => {
        this._render();
      })
      .catch((msg) => {
        if (msg.resp) {
          msg.resp.text().then(errorMessage);
        } else {
          errorMessage(msg);
        }
      })
      .finally(() => {
        if (stopSpinner) {
          this.spinner!.active = false;
        }
        window.setTimeout(() => this.poll(false), 10000);
      });
  }

  private tabSwitch(e: CustomEvent): void {
    this.state.tab = e.detail.index;
    // Unset alert_id when switching tabs.
    this.state.alert_id = '';
    this.stateHasChanged();
    // Unset filters when switching tabs.
    this.filterSilencesVal = '';
    this.filterAuditLogsVal = '';

    // If tab is stats then load stats.
    if (e.detail.index === 5) {
      this.getStats();
    }
    // If tab is silences then display empty silence to populate from scratch.
    // This will go away if any existing silence is clicked on.
    if (e.detail.index === 4) {
      fetch('/_/new_silence', {
        credentials: 'include',
      })
        .then(jsonOrThrow)
        .then((json: Silence) => {
          this.selected = null;
          this.current_silence = json;
          this.rhs_state = EDIT_SILENCE;
          this._render();
        })
        .catch(errorMessage);
    } else if (e.detail.index === 6) {
      // If tab is audit logs then load them.
      this.getAuditLogs();
      this.rhs_state = VIEW_AUDITLOG;
      this._render();
    } else {
      this.rhs_state = START;
      this._render();
    }
  }

  private clickHandler(e: KeyboardEvent): void {
    this.shift_pressed_during_click = e.shiftKey;
    e.stopPropagation();
  }

  private silenceClick(silence: Silence): void {
    this.current_silence = JSON.parse(JSON.stringify(silence)) as Silence;
    this.selected = silence;
    this.rhs_state = EDIT_SILENCE;
    this._render();
  }

  private statsClick(incident: Incident): void {
    this.selected = incident;
    this.incidentStats();
    this.rhs_state = VIEW_STATS;
  }

  private selectionModeChange(): void {
    this.inSelectionMode = !this.inSelectionMode;
    this._render();
  }

  // Update the paramset for a silence as Incidents are checked and unchecked.
  // TODO(jcgregorio) Remove this once checkbox-sk is fixed.
  private check_selected_impl(key: string, isChecked: boolean): void {
    if (isChecked) {
      this.last_checked_incident = key;
      this.checked.add(key);
      this.incidents.forEach((i) => {
        if (i.key === key) {
          paramset.add(this.current_silence!.param_set, i.params, this.ignored);
        }
      });
    } else {
      this.last_checked_incident = null;
      this.checked.delete(key);
      this.current_silence!.param_set = {};
      this.incidents.forEach((i) => {
        if (this.checked.has(i.key)) {
          paramset.add(this.current_silence!.param_set, i.params, this.ignored);
        }
      });
    }

    if (this.isBotCentricView) {
      this.make_bot_centric_param_set(this.current_silence!.param_set);
    }
    this.rhs_state = EDIT_SILENCE;
  }

  // Goes through the paramset and leaves only silence keys that are useful
  // in bot-centric view like 'alertname' and 'bot'.
  private make_bot_centric_param_set(target_paramset: ParamSet): void {
    Object.keys(target_paramset).forEach((key) => {
      if (BOT_CENTRIC_PARAMS.indexOf(key) === -1) {
        delete target_paramset[key];
      }
    });
  }

  private async check_selected(e: Event): Promise<void> {
    const checkbox = this.findParent(e.target as HTMLElement, 'CHECKBOX-SK') as CheckOrRadio;
    const incidents_to_check: string[] = [];
    if (this.isBotCentricView && this.bots_to_incidents && this.bots_to_incidents[checkbox.id]) {
      this.bots_to_incidents[checkbox.id].forEach((i) => {
        incidents_to_check.push(i.key);
      });
    } else {
      incidents_to_check.push(checkbox.id);
    }
    const checkSelectedImplFunc = () => {
      incidents_to_check.forEach((id) => {
        this.check_selected_impl(id, checkbox.checked);
      });
    };

    if (!this.checked.size) {
      // Request a new silence.
      try {
        const resp = await fetch('/_/new_silence', {
          credentials: 'include',
        });
        const silence: Silence = await jsonOrThrow(resp);
        this.selected = null;
        this.current_silence = silence;
        checkSelectedImplFunc();
      } catch (error: any) {
        errorMessage(error);
      }
    } else if (this.shift_pressed_during_click && this.last_checked_incident) {
      let foundStart = false;
      let foundEnd = false;
      // Find all incidents included in the range during shift click.
      const incidents_included_in_range: string[] = [];

      let incidents: Incident[] = [];
      if (this.state.tab === 0) {
        // Use "my" incidents if we are on the "Mine" tab.
        incidents = this.getMyIncidents();
      } else {
        // The incidents we go through for shift click selections will be
        // different for bot-centric vs normal view.
        incidents = this.isBotCentricView
          ? ([] as Incident[]).concat(...Object.values(this.bots_to_incidents))
          : this.incidents;
      }

      incidents.some((i) => {
        if (i.key === this.last_checked_incident || incidents_to_check.includes(i.key)) {
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
          this.check_selected_impl(key, true);
        });
      } else {
        // Could not find start and/or end incident. Only check the last
        // clicked.
        checkSelectedImplFunc();
      }
    } else {
      checkSelectedImplFunc();
    }
    this._render();
  }

  private select(incident: Incident): void {
    this.state.alert_id = incident.id;
    this.stateHasChanged();

    this.rhs_state = INCIDENT;
    this.checked = new Set();
    this.selected = incident;
    this.current_silence = null;
    this._render();
  }

  private addNote(e: CustomEvent): void {
    this.doImpl('/_/add_note', e.detail);
  }

  private delNote(e: CustomEvent): void {
    this.doImpl('/_/del_note', e.detail);
  }

  private addSilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you add a param.
    if (!silence.key) {
      this.current_silence = silence;
      this._render();
      return;
    }
    this.checked = new Set();
    this.doImpl('/_/save_silence', silence, (json: Silence) => this.silenceAction(json, false));
  }

  private deleteSilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you delete a param.
    if (!silence.key) {
      this.current_silence = silence;
      this._render();
      return;
    }
    this.checked = new Set();
    this.doImpl('/_/save_silence', silence, (json: Silence) => this.silenceAction(json, false));
  }

  private modifySilenceParam(silence: Silence): void {
    // Don't save silences that are just being created when you modify a param.
    if (!silence.key) {
      this.current_silence = silence;
      this._render();
      return;
    }
    this.checked = new Set();
    this.doImpl('/_/save_silence', silence, (json: Silence) => this.silenceAction(json, false));
  }

  private saveSilence(silence: Silence): void {
    this.checked = new Set();
    this.doImpl('/_/save_silence', silence, (json: Silence) => this.silenceAction(json, true));
  }

  private archiveSilence(silence: Silence): void {
    this.doImpl('/_/archive_silence', silence, (json: Silence) => this.silenceAction(json, true));
  }

  private reactivateSilence(silence: Silence): void {
    this.doImpl('/_/reactivate_silence', silence, (json: Silence) =>
      this.silenceAction(json, false)
    );
  }

  private deleteSilence(silence: Silence): void {
    this.doImpl('/_/del_silence', silence, (json: Silence) => {
      for (let i = 0; i < this.silences.length; i++) {
        if (this.silences[i].key === json.key) {
          this.silences.splice(i, 1);
          this.rhs_state = START;
          break;
        }
      }
    });
  }

  private addSilenceNote(e: CustomEvent): void {
    this.doImpl('/_/add_silence_note', e.detail, (json: Silence) =>
      this.silenceAction(json, false)
    );
  }

  private delSilenceNote(e: CustomEvent): void {
    this.doImpl('/_/del_silence_note', e.detail, (json: Silence) =>
      this.silenceAction(json, false)
    );
  }

  private botChooser(): void {
    this.populateBotsToIncidents(this.incidents);
    ($$('#bot-chooser', this) as BotChooserSk)
      .open(this.bots_to_incidents, this.current_silence!.param_set.bot!)
      .then((bot) => {
        if (!bot) {
          return;
        }
        const bot_incidents = this.bots_to_incidents[bot];
        bot_incidents.forEach((i) => {
          const bot_centric_params: Params = {};
          BOT_CENTRIC_PARAMS.forEach((p) => {
            bot_centric_params[p] = i.params[p];
          });
          paramset.add(this.current_silence!.param_set, bot_centric_params, this.ignored);
        });
        this.modifySilenceParam(this.current_silence!);
      });
  }

  private assign(e: CustomEvent): void {
    const owner = this.selected && (this.selected as Incident).params.owner;
    ($$('#email-chooser', this) as EmailChooserSk).open(this.emails, owner!).then((email) => {
      const detail = {
        key: e.detail.key,
        email: email,
      };
      this.doImpl('/_/assign', detail);
    });
  }

  private flipBotCentricView(): void {
    this.isBotCentricView = !this.isBotCentricView;
    this._render();
  }

  private assignMultiple(): void {
    // See if the selected incidents have a common owner.
    let commonOwner = '';
    for (let i = 0; i < this.incidents.length; i++) {
      if (!this.checked.has(this.incidents[i].key)) {
        // This incident has not been selected.
        continue;
      }
      const incidentOwner = this.incidents[i].params.owner;
      if (incidentOwner) {
        if (commonOwner === '') {
          commonOwner = incidentOwner;
        } else if (commonOwner !== incidentOwner) {
          // The incident owner is different than the common owner found so far.
          // This means there is no common owner;
          commonOwner = '';
          break;
        }
      } else {
        // This incident has no owner so there can be no common owner.
        commonOwner = '';
        break;
      }
    }

    ($$('#email-chooser', this) as EmailChooserSk).open(this.emails, commonOwner).then((email) => {
      const detail = {
        keys: Array.from(this.checked),
        email: email,
      };
      this.doImpl('/_/assign_multiple', detail, (json) => {
        this.incidents = json;
        this.checked = new Set();
        this._render();
      });
    });
  }

  private assignToOwner(e: CustomEvent): void {
    const owner = this.selected && (this.selected as Incident).params.owner;
    const detail = {
      key: e.detail.key,
      email: owner,
    };
    this.doImpl('/_/assign', detail);
  }

  private take(e: CustomEvent): void {
    this.doImpl('/_/take', e.detail);
    // Do not do desktop notification on takes, it is redundant.
    this.incidents_notified[e.detail.key] = true;
  }

  private getStats(): void {
    const detail: StatsRequest = {
      range: this.stats_range,
    };
    this.doImpl('/_/stats', detail, (json: Stat[]) => this.statsAction(json));
  }

  private getAuditLogs(): void {
    this.doImpl('/_/audit_logs', {}, (json: AuditLog[]) => this.auditLogsAction(json));
  }

  private incidentStats(): void {
    const detail: IncidentsInRangeRequest = {
      incident: this.selected as Incident,
      range: this.stats_range,
    };
    this.doImpl('/_/incidents_in_range', detail, (json: Incident[]) =>
      this.incidentStatsAction(json)
    );
  }

  // Actions to take after updating incident stats.
  private incidentStatsAction(json: Incident[]): void {
    this.incident_stats = json;
  }

  // Actions to take after updating Stats.
  private statsAction(json: Stat[]): void {
    this.stats = json;
  }

  // Actions to take after updating Audit Logs.
  private auditLogsAction(json: AuditLog[]): void {
    this.audit_logs = json;
  }

  // Actions to take after updating an Incident.
  private incidentAction(json: Incident): void {
    const incidents = this.incidents;
    for (let i = 0; i < incidents.length; i++) {
      if (incidents[i].key === json.key) {
        incidents[i] = json;
        break;
      }
    }
    this.selected = json;
  }

  // Actions to take after updating a Silence.
  private silenceAction(json: Silence, clear: boolean): void {
    let found = false;
    this.current_silence = json;
    for (let i = 0; i < this.silences.length; i++) {
      if (this.silences[i].key === json.key) {
        this.silences[i] = json;
        found = true;
        break;
      }
    }
    if (!found) {
      this.silences.push(json);
    }
    if (clear) {
      this.rhs_state = START;
    }
  }

  // Common work done for all fetch requests.
  private doImpl(
    url: string,
    detail: any,
    action = (json: any) => this.incidentAction(json)
  ): void {
    this.spinner!.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    })
      .then(jsonOrThrow)
      .then((json) => {
        action(json);
        this._render();
        this.spinner!.active = false;
      })
      .catch((msg) => {
        this.spinner!.active = false;
        msg.resp.text().then(errorMessage);
      });
  }

  // Fix-up all the incidents and silences, including re-sorting them.
  private rationalize(): void {
    this.incidents.forEach((incident) => {
      const silenced = this.silences.reduce(
        (isSilenced, silence) =>
          isSilenced || (silence.active && paramset.match(silence.param_set, incident.params)),
        false
      );
      incident.params.__silence_state = silenced ? 'silenced' : 'active';
    });

    // Sort the incidents, using the following 'sortby' list as tiebreakers.
    const sortby = ['__silence_state', 'assigned_to', 'alertname', 'abbr', 'id'];
    this.incidents.sort((a, b) => {
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
    this.silences.sort((a, b) => {
      // Sort active before inactive.
      if (a.active !== b.active) {
        return a.active ? -1 : 1;
      }
      return b.updated - a.updated;
    });
  }

  private needsTriaging(
    incident: Incident,
    isInfraGardener: boolean,
    isSkiaGardener: boolean
  ): boolean {
    if (
      incident.active &&
      incident.params.__silence_state !== 'silenced' &&
      ((isInfraGardener &&
        incident.params['category'] === 'infra' &&
        !incident.params.assigned_to) ||
        (isSkiaGardener &&
          incident.params['category'] === 'skia' &&
          !incident.params.assigned_to) ||
        incident.params.assigned_to === this.user ||
        (incident.params.owner === this.user && !incident.params.assigned_to))
    ) {
      return true;
    }
    return false;
  }

  private requestDesktopNotificationPermission(): void {
    if (Notification && Notification.permission === 'default') {
      Notification.requestPermission();
    }
  }

  private sendDesktopNotification(unNotifiedIncidents: Incident[]): void {
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
      icon: '/dist/icon-active.png',
      body: text,
      // 'tag' handles multi-tab scenarios. When multiple tabs are open then
      // only one notification is sent for the same alert.
      tag: `alertManagerNotification${text}`,
    });
    // onclick move focus to the am.skia.org tab and close the notification.
    notification.onclick = () => {
      window.parent.focus();
      window.focus(); // Supports older browsers.
      this.select(unNotifiedIncidents[0]); // Display the 1st incident.
      notification.close();
    };
    setTimeout(notification.close.bind(notification), 10000);
  }

  private _render(): void {
    this.rationalize();
    render(AlertManagerSk.template(this), this, { host: this });
    // Update the icon.
    const isInfraGardener = this.user === this.infra_gardener;
    const isSkiaGardener = this.user === this.skia_gardener;
    const numActive = this.incidents.reduce(
      (n, incident) => (n += this.needsTriaging(incident, isInfraGardener, isSkiaGardener) ? 1 : 0),
      0
    );

    // Show desktop notifications only if permission was granted and only if
    // silences have been successfully fetched. If silences have not been
    // fetched yet then we might end up notifying on silenced incidents.
    if (Notification.permission === 'granted' && this.silences.length !== 0) {
      const unNotifiedIncidents = this.incidents.filter(
        (i) =>
          !this.incidents_notified[i.key] && this.needsTriaging(i, isInfraGardener, isSkiaGardener)
      );
      this.sendDesktopNotification(unNotifiedIncidents);
      unNotifiedIncidents.forEach((i) => (this.incidents_notified[i.key] = true));
    }

    document.title = `${numActive} - AlertManager`;
    if (!this.favicon) {
      return;
    }
    if (numActive > 0) {
      this.favicon.href = '/dist/icon-active.png';
    } else {
      this.favicon.href = '/dist/icon.png';
    }
  }
}

define('alert-manager-sk', AlertManagerSk);

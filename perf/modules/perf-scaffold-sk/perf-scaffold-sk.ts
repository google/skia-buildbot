/**
 * @module module/perf-scaffold-sk
 * @description <h2><code>perf-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the Perf pages. The rest of
 * every Perf page should be a child of this element.
 *
 */
import { html, LitElement, nothing, TemplateResult } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { choose } from 'lit/directives/choose.js';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/add-alert-icon-sk';
import '../../../elements-sk/modules/icons/build-icon-sk';
import '../../../elements-sk/modules/icons/bug-report-icon-sk';
import '../../../elements-sk/modules/icons/compare-arrows-icon-sk';
import '../../../elements-sk/modules/icons/event-icon-sk';
import '../../../elements-sk/modules/icons/favorite-icon-sk';
import '../../../elements-sk/modules/icons/folder-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import '../../../elements-sk/modules/icons/home-icon-sk';
import '../../../elements-sk/modules/icons/multiline-chart-icon-sk';
import '../../../elements-sk/modules/icons/sort-icon-sk';
import '../../../elements-sk/modules/icons/trending-up-icon-sk';
import '../../../elements-sk/modules/icons/launch-icon-sk';
import '../../../elements-sk/modules/icons/chat-icon-sk';
import '../../../elements-sk/modules/icons/lightbulb-outline-icon-sk';
import '../../../elements-sk/modules/icons/settings-backup-restore-icon-sk';
import '../../../elements-sk/modules/icons/autorenew-icon-sk';
import '../../../infra-sk/modules/alogin-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import '../gemini-side-panel-sk/gemini-side-panel-sk';
import { GeminiSidePanelSk } from '../gemini-side-panel-sk/gemini-side-panel-sk';
import { getBuildTag } from '../window/window';

// The ID of a top level element under perf-scaffold-sk that will be moved under
// the help dropdown (new UI) or right hand side nav bar (old UI).
const SIDEBAR_HELP_ID = 'sidebar_help';

const BUILDBOT_GIT = 'https://skia.googlesource.com/buildbot.git/+log/';
const SKIA_INFRA_REPO = 'https://skia.googlesource.com/buildbot.git';
const PINPOINT_URL = 'https://pinpoint-dot-chromeperf.appspot.com/';
const COOKIE_MISC = 'path=/; max-age=31536000; SameSite=Strict;';

@customElement('perf-scaffold-sk')
export class PerfScaffoldSk extends LitElement {
  @state()
  private hasHelpContent: boolean = false;

  @state()
  private isHiddenTriage: boolean =
    window.perf.show_triage_link !== null ? window.perf.show_triage_link : true;

  @state()
  private _isRefreshing: boolean = false;

  @state()
  private _devVersion: number = 0;

  @state()
  private _helpUrl: string = 'http://go/perf-user-doc';

  @state()
  private _reportBugUrl: string =
    'https://issuetracker.google.com/issues/new?component=1547614&template=1970127';

  @state()
  private _contentNodes: Node[] = [];

  private get isV2Enabled(): boolean {
    const localPref = localStorage.getItem('perf:use-explore-v2');
    if (localPref !== null) {
      return localPref === 'true';
    }
    return !!window.perf.default_to_explore_v2;
  }

  private get multigraphUrl(): string {
    if (this.isV2Enabled) {
      return '/e2';
    }
    if (window.perf.default_to_manual_plot_mode) {
      return '/m?manual_plot_mode=true';
    }
    return '/m';
  }

  constructor() {
    super();
    // Override window.perf if a local preference exists.
    const fetchAnomaliesFromSqlStr = localStorage.getItem('fetch_anomalies_from_sql');
    if (fetchAnomaliesFromSqlStr !== null) {
      window.perf.fetch_anomalies_from_sql = fetchAnomaliesFromSqlStr === 'true';
      window.perf.fetch_chrome_perf_anomalies = !window.perf.fetch_anomalies_from_sql;
    }
    // Show legacy anomalies by default. We have to explicitly set that on the first visit.
    if (
      window.perf.both_anomaly_sources &&
      window.perf.fetch_anomalies_from_sql &&
      window.perf.fetch_chrome_perf_anomalies
    ) {
      // sql and chromeperf are exclusive, and we rely on both_sources flag later.
      window.perf.fetch_anomalies_from_sql = false;
    }
    // Set cookie for backend processing
    const cookie = `fetch_anomalies_from_sql=${window.perf.fetch_anomalies_from_sql}`;
    document.cookie = `${cookie}; ${COOKIE_MISC}`;
  }

  createRenderRoot() {
    return this;
  }

  private get anomalySourceText(): string {
    return (
      'Currently showing: ' +
      (window.perf.fetch_anomalies_from_sql ? 'New anomalies' : 'Legacy (Chromeperf) anomalies')
    );
  }

  private fallbackLogo(e: Event) {
    const img = e.target as HTMLImageElement;
    if (img.src.indexOf('/dist/images/alpine_transparent.png') >= 0) {
      return;
    }
    img.src = '/dist/images/alpine_transparent.png';
  }

  private toggleAnomaliesSource(_e: Event) {
    const isChecked = !window.perf.fetch_anomalies_from_sql;
    window.perf.fetch_anomalies_from_sql = isChecked;
    // ensure exclusivity based on selection
    window.perf.fetch_chrome_perf_anomalies = !isChecked;

    // SAVE TO LOCAL STORAGE
    localStorage.setItem('fetch_anomalies_from_sql', isChecked.toString());
    // SAVE TO COOKIE
    document.cookie = `fetch_anomalies_from_sql=${isChecked}; ${COOKIE_MISC}`;

    window.dispatchEvent(
      new CustomEvent('anomalies-source-changed', {
        detail: { fetch_anomalies_from_sql: window.perf.fetch_anomalies_from_sql },
      })
    );
    this.requestUpdate();
  }

  render() {
    const isV2 = this.isV2();

    const helpNodes = this._contentNodes.filter((n) => (n as HTMLElement).id === SIDEBAR_HELP_ID);
    const mainNodes = this._contentNodes.filter((n) => (n as HTMLElement).id !== SIDEBAR_HELP_ID);

    if (isV2) {
      return this.renderV2UI(mainNodes, helpNodes);
    }
    return this.renderLegacyUI(mainNodes, helpNodes);
  }

  private isV2(): boolean {
    const isV2Default = window.perf.enable_v2_ui;
    const storedPreference = localStorage.getItem('v2_ui');
    return storedPreference === 'true' || (storedPreference === null && isV2Default);
  }

  private renderLegacyUI(mainNodes: Node[], helpNodes: Node[]) {
    return html`
      <app-sk class="legacy-ui">
        <header id="topbar">
          <div class="header-brand">
            <a href="/">
              <img
                class="logo"
                src="${window.perf.header_image_url || '/dist/images/alpine_transparent.png'}"
                alt="Logo"
                @error=${this.fallbackLogo} />
            </a>
          </div>
          <h1 class="name">${this.instanceTitleTemplate()}</h1>
          <div class="spacer"></div>
          <button
            ?hidden=${!window.perf.both_anomaly_sources}
            @click=${this.toggleAnomaliesSource}
            title="Toggle Anomaly Source"
            class="anomaly-toggle">
            ${this.anomalySourceText}
          </button>
          <alogin-sk url="/_/login/status"></alogin-sk>
          <theme-chooser-sk></theme-chooser-sk>
        </header>
        <aside id="sidebar">
          <div id="links">
            <a href="/e" tab-index="0"><home-icon-sk></home-icon-sk><span>New Query</span></a>
            <a href="/f" tab-index="0"
              ><favorite-icon-sk></favorite-icon-sk><span>Favorites</span></a
            >
            <a href="${this.multigraphUrl}" tab-index="0">
              <multiline-chart-icon-sk></multiline-chart-icon-sk><span>MultiGraph</span>
            </a>
            ${!this.isV2Enabled
              ? html`
                  <a href="/e2" tab-index="0">
                    <multiline-chart-icon-sk></multiline-chart-icon-sk
                    ><span>Graphs (Prototype)</span>
                  </a>
                `
              : ''}
            <div ?hidden=${!window.perf.extra_links}>
              <a href="/l" tab-index="0">
                <compare-arrows-icon-sk></compare-arrows-icon-sk>
                <span>${window.perf.extra_links?.name}</span>
              </a>
            </div>
            <div class="triage-link" ?hidden=${!this.isHiddenTriage}>
              <a href="/t" tab-index="0"
                ><trending-up-icon-sk></trending-up-icon-sk><span>Triage</span></a
              >
            </div>
            <a href="/a" tab-index="0"
              ><add-alert-icon-sk></add-alert-icon-sk><span>Alerts</span></a
            >
            <a href="/d" tab-index="0"><build-icon-sk></build-icon-sk><span>Dry Run</span></a>
            <a href="/c" tab-index="0"><sort-icon-sk></sort-icon-sk><span>Clustering</span></a>
            ${PerfScaffoldSk.revisionLinkTemplateOld()}
            <a href="${this._helpUrl}" target="_blank" tab-index="0">
              <help-icon-sk></help-icon-sk><span>Help</span>
            </a>
            <a href="${this._reportBugUrl}" target="_blank" tab-index="0">
              <bug-report-icon-sk></bug-report-icon-sk><span>Report Bug</span>
            </a>
            ${this.appVersionTemplate()}
          </div>
          <div id="help">${helpNodes}</div>
          <div id="chat">${this.chatLinkTemplate()}</div>
          <button @click=${() => this.toggleUI(true)} class="try-v2-ui">Try V2 UI</button>
        </aside>
        <main id="perf-content">${mainNodes}</main>
        <footer class="glue-footer">
          <error-toast-sk></error-toast-sk>
        </footer>
      </app-sk>
    `;
  }

  private renderV2UI(mainNodes: Node[], helpNodes: Node[]) {
    return html`
      <app-sk class="v2-ui">
        <header id="topbar">
          <a class="header-brand" href="/">
            <img
              src="${window.perf.header_image_url || '/dist/images/alpine_transparent.png'}"
              alt="Logo"
              class="logo"
              @error=${this.fallbackLogo} />
            <h1 class="name">${this.instanceTitleTemplate()}</h1>
          </a>
          <nav id="header-nav-items">
            ${!window.perf.default_to_explore_v2
              ? html`
                  <a href="/e" tab-index="0" class="${this.isPageActive('/e') ? 'active' : ''}"
                    >Explore</a
                  >
                `
              : ''}
            <a
              href="${this.multigraphUrl}"
              tab-index="0"
              class="${this.isPageActive(this.isV2Enabled ? '/e2' : '/m') ? 'active' : ''}"
              >MultiGraph</a
            >
            ${!this.isV2Enabled
              ? html`
                  <a href="/e2" tab-index="0" class="${this.isPageActive('/e2') ? 'active' : ''}"
                    >Graphs (Prototype)</a
                  >
                `
              : ''}
            <div class="triage-link" ?hidden=${!this.isHiddenTriage}>
              <a href="/t" tab-index="0" class="${this.isPageActive('/t') ? 'active' : ''}"
                >Triage</a
              >
            </div>
            <a href="/a" tab-index="0" class="${this.isPageActive('/a') ? 'active' : ''}">Alerts</a>
            <a href="/f" tab-index="0" class="${this.isPageActive('/f') ? 'active' : ''}"
              >Favorites</a
            >
            <div ?hidden=${!window.perf.extra_links}>
              <a href="/l" tab-index="0" class="${this.isPageActive('/l') ? 'active' : ''}">
                ${window.perf.extra_links?.name}
              </a>
            </div>
            <a href="/d" tab-index="0" class="${this.isPageActive('/d') ? 'active' : ''}"
              >Dry Run</a
            >
            <a href="/c" tab-index="0" class="${this.isPageActive('/c') ? 'active' : ''}"
              >Clustering</a
            >
            <a href="/pg" tab-index="0" class="${this.isPageActive('/pg') ? 'active' : ''}"
              >Playground</a
            >
            ${PerfScaffoldSk.revisionLinkTemplateNew(this)}
            <a href="${PINPOINT_URL}" target="_blank" tab-index="0">
              Pinpoint
              <launch-icon-sk></launch-icon-sk>
            </a>
          </nav>
          <div id="header-aside-container">
            <div id="header-aside">
              <button
                ?hidden=${!window.perf.both_anomaly_sources}
                @click=${this.toggleAnomaliesSource}
                title="Toggle Anomaly Source"
                class="aside-button anomaly-toggle">
                ${this.anomalySourceText}
              </button>
              <a
                href="${this._reportBugUrl}"
                target="_blank"
                tab-index="0"
                title="Report Bug"
                class="aside-button">
                <bug-report-icon-sk></bug-report-icon-sk>
              </a>
              ${this.chatLinkTemplate()}
              <button @click=${this.toggleGemini} title="Ask Gemini" class="aside-button">
                <lightbulb-outline-icon-sk></lightbulb-outline-icon-sk>
              </button>
              <button id="help-button" @click=${this.toggleHelp} title="Help" class="aside-button">
                <help-icon-sk></help-icon-sk>
              </button>
              <button
                id="legacy-ui-button"
                @click=${() => this.toggleUI(false)}
                title="Back to Legacy UI"
                class="aside-button">
                <settings-backup-restore-icon-sk></settings-backup-restore-icon-sk>
              </button>
              <alogin-sk url="/_/login/status"></alogin-sk>
              <theme-chooser-sk></theme-chooser-sk>
            </div>
          </div>
          <div id="help-dropdown" class="hidden">
            <a href="${this._helpUrl}" target="_blank" class="help-link">
              <span>Documentation</span>
              <launch-icon-sk></launch-icon-sk>
            </a>
            <hr ?hidden=${!this.hasHelpContent} />
            <div id="help-content">${helpNodes}</div>
          </div>
        </header>
        <main id="perf-content">${mainNodes}</main>
        <gemini-side-panel-sk></gemini-side-panel-sk>
        <footer class="glue-footer">
          <error-toast-sk></error-toast-sk>
          ${this.buildTagTemplate()}
        </footer>
      </app-sk>
    `;
  }

  private static revisionLinkTemplateOld = () => {
    if (window.perf.fetch_chrome_perf_anomalies || window.perf.fetch_anomalies_from_sql) {
      return html`<a href="/v" tab-index="0"
        ><trending-up-icon-sk></trending-up-icon-sk><span>Revision Info</span></a
      >`;
    }
    return html``;
  };

  private static revisionLinkTemplateNew = (ele: PerfScaffoldSk) => {
    if (window.perf.fetch_chrome_perf_anomalies || window.perf.fetch_anomalies_from_sql) {
      return html`<a href="/v" tab-index="0" class="${ele.isPageActive('/v') ? 'active' : ''}"
        >Revision Info</a
      >`;
    }
    return html``;
  };

  private appVersionTemplate() {
    const appVersion = window.perf.app_version || `dev-${new Date().toISOString()}`;
    const buildDate = window.perf.build_date;

    const dateStr = appVersion.startsWith('dev-') ? appVersion.substring(4) : appVersion;
    const date = new Date(dateStr);

    let buildDateTemplate: TemplateResult | typeof nothing = nothing;
    if (buildDate) {
      buildDateTemplate = html`<div class="build-date" title="Build Date">
        Build: ${buildDate}
      </div>`;
    }

    const schemaVersion = window.perf.schema_version;
    let schemaVersionTemplate: TemplateResult | typeof nothing = nothing;
    if (schemaVersion) {
      schemaVersionTemplate = html`<div
        class="schema-version"
        title="Currently applied database schema version">
        Schema: v${schemaVersion}
      </div>`;
    }

    if (!isNaN(date.getTime()) && dateStr.includes('-') && dateStr.includes(':')) {
      const pad = (n: number) => n.toString().padStart(2, '0');

      const formattedDate = `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(
        date.getUTCDate()
      )} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`;

      return html`
        <div class="version-container">
          <a class="version" title="${appVersion}">
            <span>dev-build (${formattedDate})</span>
          </a>
          ${schemaVersionTemplate}
        </div>
      `;
    }

    // 2. Treat as git hash (long or short)

    const shortHash = appVersion.length >= 7 ? appVersion.substring(0, 7) : appVersion;

    return html`
      <div class="version-container">
        <a
          class="version"
          href="${SKIA_INFRA_REPO}/+/${appVersion}"
          target="_blank"
          title="${appVersion}">
          <span>Ver: ${shortHash}</span>
        </a>
        ${buildDateTemplate} ${schemaVersionTemplate}
      </div>
    `;
  }

  private chatLinkTemplate() {
    if (window.perf.chat_url) {
      if (this.isV2()) {
        return html`<a
          href="${window.perf.chat_url}"
          target="_blank"
          tab-index="0"
          class="aside-button"
          title="Ask the team">
          <chat-icon-sk></chat-icon-sk>
        </a>`;
      }
      return html`<a target="_blank" href="${window.perf.chat_url}" tab-index="0"
        ><h4>Ask the team</h4></a
      >`;
    }
    return html``;
  }

  private buildTagLinkTemplate(tag: string) {
    return html`<a href="${BUILDBOT_GIT}${tag}" target="_blank" class="dashboard-version"
      >Build: ${tag}</a
    >`;
  }

  private buildTagTemplate() {
    if (window.perf.dev_mode) {
      return html` <a class="dashboard-version dev-mode">
        <autorenew-icon-sk
          class="${this._isRefreshing ? 'refreshing' : ''}"
          @click=${() => this._reload()}
          title="Force reload"></autorenew-icon-sk>
        ${window.perf.image_tag}
      </a>`;
    }
    const buildTag = getBuildTag();
    return html`${choose(
      buildTag.type,
      [
        ['git', () => this.buildTagLinkTemplate(buildTag.tag!)],
        ['louhi', () => this.buildTagLinkTemplate(buildTag.tag!)],
        ['tag', () => html`<a class="dashboard-version">Build: ${buildTag.tag}</a>`],
      ],
      () => html`<a class="dashboard-version">Build: No Tag</a>`
    )}`;
  }

  private instanceTitleTemplate() {
    // Use the instance name from the config if it's available.
    // This allows us to set a custom name that might not match the URL.
    if (window.perf.instance_name) {
      let name = window.perf.instance_name;
      if (name.length > 64) {
        name = name.substring(0, 64);
      }
      return html`<a>${name}</a>`;
    }
    // Fallback to extracting the name from the URL if no config name is provided.
    if (window.perf.instance_url) {
      if (window.perf.enable_v2_ui && localStorage.getItem('v2_ui') === 'true') {
        return html`${this.extractInstanceNameFromUrl(window.perf.instance_url)}`;
      }
      return html`<a>${this.extractInstanceNameFromUrl(window.perf.instance_url)}</a>`;
    }
  }

  private extractInstanceNameFromUrl(url: string): string {
    const regex = /^https:\/\/(.*?)\./;
    const match = url.match(regex);
    if (match && match[1]) {
      return match[1].at(0)!.toUpperCase() + match[1].substring(1);
    }
    return '';
  }

  private checkDevVersion() {
    if (this._devVersion !== 0) {
      return;
    }
    fetch('/_/dev/version')
      .then((resp) => resp.json())
      .then((json) => {
        this._devVersion = json.version;
      })
      .catch(() => {});
  }

  connectedCallback(): void {
    super.connectedCallback();

    const isV2 = this.isV2();

    if (isV2) {
      this.injectFavicon();
      this.updateTitle();
    }

    if (window.perf.help_url_override && window.perf.help_url_override !== '') {
      this._helpUrl = window.perf.help_url_override;
    }
    if (window.perf.feedback_url && window.perf.feedback_url !== '') {
      this._reportBugUrl = window.perf.feedback_url;
    }

    if (window.perf.dev_mode) {
      this.checkDevVersion();

      if (window.EventSource) {
        const source = new EventSource('/__livereload');
        source.onerror = function (e) {
          console.debug('livereload connection error', e);
        };
        source.onmessage = function (e) {
          if (e.data === 'css') {
            document.querySelectorAll('link[rel="stylesheet"]').forEach((link) => {
              const url = new URL((link as HTMLLinkElement).href, window.location.origin);
              url.searchParams.set('v', new Date().getTime().toString());
              (link as HTMLLinkElement).href = url.href;
            });
            console.log('Styles hot-swapped');
          } else {
            console.log('Code changed. Full reload...');
            window.location.reload();
          }
        };
      }
    }

    // Save children before Lit renders over them.
    this._contentNodes = Array.prototype.slice.call(this.childNodes);
    this.innerHTML = ''; // Clear them out so Lit doesn't find them when it renders its template into `this`.

    const observer = new MutationObserver((mutList) => {
      let changed = false;
      mutList.forEach((mut) => {
        mut.addedNodes.forEach((node) => {
          if (node.parentElement === this && (node as HTMLElement).tagName !== 'APP-SK') {
            this._contentNodes.push(node);
            changed = true;
            node.parentNode?.removeChild(node); // Remove from direct children
          }
        });
      });
      if (changed) {
        this.requestUpdate();
      }
    });
    observer.observe(this, { childList: true });
  }

  private injectFavicon() {
    let link = document.querySelector("link[rel~='icon']") as HTMLLinkElement;
    if (!link) {
      link = document.createElement('link');
      link.rel = 'icon';
      document.head.appendChild(link);
    }
    link.type = 'image/svg+xml';
    link.href = '/dist/images/line-chart.svg';
  }

  private updateTitle() {
    const instanceName = this.extractInstanceNameFromUrl(window.perf.instance_url);
    if (instanceName) {
      document.title = `${instanceName} Performance Monitoring`;
    }
  }

  private isPageActive(url: string): boolean {
    return window.location.pathname.startsWith(url);
  }

  private toggleHelp() {
    const dropdown = this.querySelector('#help-dropdown');
    if (dropdown) {
      dropdown.classList.toggle('hidden');
    }
  }

  private toggleGemini() {
    const gemini = this.querySelector('gemini-side-panel-sk') as GeminiSidePanelSk;
    if (gemini) {
      gemini.toggle();
    }
  }

  private toggleUI(enable: boolean) {
    localStorage.setItem('v2_ui', String(enable));
    this._reload();
  }

  private _reload() {
    window.location.reload();
  }
}

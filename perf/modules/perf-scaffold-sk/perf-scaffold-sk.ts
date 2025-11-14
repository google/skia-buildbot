/**
 * @module module/perf-scaffold-sk
 * @description <h2><code>perf-scaffold-sk</code></h2>
 *
 * Contains the title bar and error-toast for all the Perf pages. The rest of
 * every Perf page should be a child of this element.
 *
 */
import { html } from 'lit/html.js';
import { choose } from 'lit/directives/choose.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/add-alert-icon-sk';
import '../../../elements-sk/modules/icons/build-icon-sk';
import '../../../elements-sk/modules/icons/bug-report-icon-sk';
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
import { getBuildTag } from '../window/window';

// The ID of a top level element under perf-scaffold-sk that will be moved under
// the help dropdown (new UI) or right hand side nav bar (old UI).
const SIDEBAR_HELP_ID = 'sidebar_help';

const BUILDBOT_GIT = 'https://skia.googlesource.com/buildbot.git/+log/';
const SKIA_INFRA_REPO = 'https://skia.googlesource.com/buildbot.git';
const PINPOINT_URL = 'https://pinpoint-dot-chromeperf.appspot.com/';
/**
 * Moves the elements from a list to be the children of the target element.
 *
 * @param from - The list of elements we are moving.
 * @param to - The new parent.
 */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class PerfScaffoldSk extends ElementSk {
  private _main: HTMLElement | null = null;

  private _help: HTMLElement | null = null;

  private _chat: HTMLElement | null = null;

  private hasHelpContent: boolean = false;

  private _helpUrl: string = 'http://go/perf-user-doc';

  private _reportBugUrl: string =
    'https://issuetracker.google.com/issues/new?component=1547614&template=1970127';

  private isHiddenTriage =
    window.perf.show_triage_link !== null ? window.perf.show_triage_link : true;

  private _isRefreshing: boolean = false;

  private _pollInterval: number | null = null;

  constructor() {
    super(PerfScaffoldSk.template);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._pollInterval) {
      window.clearInterval(this._pollInterval);
      this._pollInterval = null;
    }
  }

  private static template = (ele: PerfScaffoldSk) => {
    const isV2Default = window.perf.enable_v2_ui;
    const storedPreference = localStorage.getItem('v2_ui');

    if (storedPreference === 'true' || (storedPreference === null && isV2Default)) {
      return ele.renderV2UI(ele);
    }
    return ele.renderLegacyUI(ele);
  };

  private fallbackLogo(e: Event) {
    const img = e.target as HTMLImageElement;
    // Prevent infinite loop if the default image also fails
    if (img.src.indexOf('/dist/images/alpine_transparent.png') >= 0) {
      return;
    }
    img.src = '/dist/images/alpine_transparent.png';
  }

  private renderLegacyUI(ele: PerfScaffoldSk) {
    return html`
  <app-sk class="legacy-ui">
    <header id=topbar>
      <div class="header-brand">
        <a href="/">
          <img class="logo" src="${
            window.perf.header_image_url || '/dist/images/alpine_transparent.png'
          }" alt="Logo" @error=${ele.fallbackLogo} />
        </a>
      </div>
      <h1 class=name>${ele.instanceTitleTemplate()}</h1>
      <div class=spacer></div>
      <alogin-sk url=/_/login/status></alogin-sk>
      <theme-chooser-sk></theme-chooser-sk>
    </header>
    <aside id=sidebar>
      <div id=links>
        <a href="/e" tab-index=0 ><home-icon-sk></home-icon-sk><span>New Query</span></a>
        <a href="/f" tab-index=0 ><favorite-icon-sk></favorite-icon-sk><span>Favorites</span></a>
        <a href="/m" tab-index=0 >
          <multiline-chart-icon-sk></multiline-chart-icon-sk><span>MultiGraph</span>
        </a>
        <div class="triage-link" ?hidden=${!ele.isHiddenTriage}>
        <a href="/t" tab-index=0 ><trending-up-icon-sk></trending-up-icon-sk><span>Triage</span></a>
        </div>
        <a href="/a" tab-index=0 ><add-alert-icon-sk></add-alert-icon-sk><span>Alerts</span></a>
        <a href="/d" tab-index=0 ><build-icon-sk></build-icon-sk><span>Dry Run</span></a>
        <a href="/c" tab-index=0 ><sort-icon-sk></sort-icon-sk><span>Clustering</span></a>
        ${PerfScaffoldSk.revisionLinkTemplateOld()}
        <a href="${ele._helpUrl}" target="_blank" tab-index=0 >
          <help-icon-sk></help-icon-sk><span>Help</span>
        </a>
        <a href="${ele._reportBugUrl}" target="_blank" tab-index=0 >
          <bug-report-icon-sk></bug-report-icon-sk><span>Report Bug</span>
        </a>
        ${ele.appVersionTemplate()}
      </div>
      <div id=help>
      </div>
      <div id=chat>
      </div>
      <button @click=${() => ele.toggleUI(true)} class="try-v2-ui">Try V2 UI</button>
    </aside>
    <main id="perf-content">
    </main>
    <footer class="glue-footer">
      <error-toast-sk></error-toast-sk>
    </footer>
  </app-sk>
`;
  }

  private renderV2UI(ele: PerfScaffoldSk) {
    return html`
  <app-sk class="v2-ui">
    <header id=topbar>
      <a class="header-brand" href="/">
        <img src="${
          window.perf.header_image_url || '/dist/images/alpine_transparent.png'
        }" alt="Logo" class="logo" @error=${ele.fallbackLogo}>
        <h1 class=name>${ele.instanceTitleTemplate()}</h1>
      </a>
      <nav id="header-nav-items">
        <a href="/e" tab-index=0 class="${ele.isPageActive('/e') ? 'active' : ''}">Explore</a>
        <a href="/m" tab-index=0 class="${ele.isPageActive('/m') ? 'active' : ''}">MultiGraph</a>
        <div class="triage-link" ?hidden=${!ele.isHiddenTriage}>
          <a href="/t" tab-index=0 class="${ele.isPageActive('/t') ? 'active' : ''}">Triage</a>
        </div>
        <a href="/a" tab-index=0 class="${ele.isPageActive('/a') ? 'active' : ''}">Alerts</a>
        <a href="/f" tab-index=0 class="${ele.isPageActive('/f') ? 'active' : ''}">Favorites</a>
        <a href="/d" tab-index=0 class="${ele.isPageActive('/d') ? 'active' : ''}">Dry Run</a>
        <a href="/c" tab-index=0 class="${ele.isPageActive('/c') ? 'active' : ''}">Clustering</a>
        ${PerfScaffoldSk.revisionLinkTemplateNew(ele)}
        <a href="${PINPOINT_URL}" target="_blank" tab-index="0">
          Pinpoint
          <launch-icon-sk></launch-icon-sk>
        </a>
      </nav>
      <div id="header-aside-container">
        <div id="header-aside">
          <a href="${
            ele._reportBugUrl
          }" target="_blank" tab-index=0 title="Report Bug" class="aside-button">
            <bug-report-icon-sk></bug-report-icon-sk>
          </a>
          ${ele.chatLinkTemplate()}
          <button id="help-button" @click=${ele.toggleHelp} title="Help" class="aside-button">
            <help-icon-sk></help-icon-sk>
          </button>
          <button id="legacy-ui-button" @click=${() =>
            ele.toggleUI(false)} title="Back to Legacy UI" class="aside-button">
            <settings-backup-restore-icon-sk></settings-backup-restore-icon-sk>
          </button>
          <alogin-sk url=/_/login/status></alogin-sk>
          <theme-chooser-sk></theme-chooser-sk>
        </div>
      </div>
      <div id="help-dropdown" class="hidden">
        <a href="${ele._helpUrl}" target="_blank" class="help-link">
          <span>Documentation</span>
          <launch-icon-sk></launch-icon-sk>
        </a>
        <hr ?hidden=${!ele.hasHelpContent} />
        <div id="help-content"></div>
      </div>
    </header>
    <main id="perf-content">
    </main>
    <footer class="glue-footer">
      <error-toast-sk></error-toast-sk>
      ${ele.buildTagTemplate()}
    </footer>
  </app-sk>
`;
  }

  private static revisionLinkTemplateOld = () => {
    if (window.perf.fetch_chrome_perf_anomalies) {
      return html`<a href="/v" tab-index="0"
        ><trending-up-icon-sk></trending-up-icon-sk><span>Revision Info</span></a
      >`;
    }
    return html``;
  };

  private static revisionLinkTemplateNew = (ele: PerfScaffoldSk) => {
    if (window.perf.fetch_chrome_perf_anomalies) {
      return html`<a href="/v" tab-index="0" class="${ele.isPageActive('/v') ? 'active' : ''}"
        >Revision Info</a
      >`;
    }
    return html``;
  };

  private appVersionTemplate() {
    const appVersion = window.perf.app_version || `dev-${new Date().toISOString()}`;

    // 1. Try to parse as a dev date (with or without 'dev-' prefix)

    const dateStr = appVersion.startsWith('dev-') ? appVersion.substring(4) : appVersion;

    const date = new Date(dateStr);

    // Check if it's a valid date and looks like a timestamp (e.g. has '-' and ':')

    // to avoid false positives with some hash-like strings that might parse as dates.

    if (!isNaN(date.getTime()) && dateStr.includes('-') && dateStr.includes(':')) {
      const pad = (n: number) => n.toString().padStart(2, '0');

      const formattedDate = `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(
        date.getUTCDate()
      )} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`;

      return html`<a class="version" title="${appVersion}"
        ><span>dev-build (${formattedDate})</span></a
      >`;
    }

    // 2. Treat as git hash (long or short)

    const shortHash = appVersion.length >= 7 ? appVersion.substring(0, 7) : appVersion;

    return html`<a
      class="version"
      href="${SKIA_INFRA_REPO}/+/${appVersion}"
      target="_blank"
      title="${appVersion}">
      <span>Ver: ${shortHash}</span>
    </a>`;
  }

  private chatLinkTemplate() {
    if (window.perf.chat_url) {
      return html`<a
        href="${window.perf.chat_url}"
        target="_blank"
        tab-index="0"
        class="aside-button"
        title="Ask the team">
        <chat-icon-sk></chat-icon-sk>
      </a>`;
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

  private startDevPoll() {
    let currentVersion = 0;
    this._pollInterval = window.setInterval(() => {
      fetch('/_/dev/version')
        .then((resp) => resp.json())
        .then((json) => {
          if (currentVersion === 0) {
            currentVersion = json.version;
          } else if (currentVersion !== json.version) {
            this._isRefreshing = true;
            this._render();
            this._reload();
          }
        })
        .catch(() => {});
    }, 2000);
  }

  connectedCallback(): void {
    super.connectedCallback();
    if (this._main) {
      return;
    }

    if (window.perf.enable_v2_ui && localStorage.getItem('v2_ui') === 'true') {
      this.injectFavicon();
      this.updateTitle();
    }

    const div = document.createElement('div');
    move(this.children, div);

    if (window.perf.help_url_override && window.perf.help_url_override !== '') {
      this._helpUrl = window.perf.help_url_override;
    }
    if (window.perf.feedback_url && window.perf.feedback_url !== '') {
      this._reportBugUrl = window.perf.feedback_url;
    }

    if (window.perf.dev_mode) {
      this.startDevPoll();
    }

    this._render();

    // Use a specific ID to avoid finding <main> inside other components
    // (like gemini-side-panel-sk).
    this._main = this.querySelector('#perf-content');
    if (!this._main) {
      console.error('perf-scaffold-sk: #perf-content not found after _render()');
    }
    this._help = this.querySelector('#help-content') || this.querySelector('#help');
    this._chat = this.querySelector('#chat');

    if (this._chat !== null) {
      this.addUrlToElement(this._chat, 'Ask the team', window.perf.chat_url);
    }

    this.redistributeAddedNodes(div.childNodes);

    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        this.redistributeAddedNodes(mut.addedNodes);
      });
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

  private addUrlToElement(element: HTMLElement, urlText: string, urlHref: string) {
    element.hidden = true;
    if (urlHref && urlHref !== '') {
      element.innerHTML = `<a target="_blank" href="${urlHref}"
        tab-index=0><h4>${urlText}</h4></a>`;
      element.hidden = false;
    }
  }

  private redistributeAddedNodes(from: NodeList) {
    if (!this._main) {
      this._main = this.querySelector('#perf-content');
    }
    Array.prototype.slice.call(from).forEach((node: Node) => {
      if ((node as Element).id === SIDEBAR_HELP_ID) {
        if (this._help) {
          this._help.appendChild(node);
          if (window.perf.enable_v2_ui && localStorage.getItem('v2_ui') === 'true') {
            this.hasHelpContent = true;
            this._render();
          }
        }
      } else {
        if (this._main) {
          this._main.appendChild(node);
        } else {
          console.error('perf-scaffold-sk: main element not found for node', node);
        }
      }
    });
  }

  private toggleHelp() {
    const dropdown = this.querySelector('#help-dropdown');
    if (dropdown) {
      dropdown.classList.toggle('hidden');
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

define('perf-scaffold-sk', PerfScaffoldSk);

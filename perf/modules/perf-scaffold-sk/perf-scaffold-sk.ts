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
import '../../../infra-sk/modules/alogin-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import { getBuildTag } from '../window/window';

// The ID of a top level element under perf-scaffold-sk that will be moved under
// the right hand side nav bar.
const SIDEBAR_HELP_ID = 'sidebar_help';

const BUILDBOT_GIT = 'https://skia.googlesource.com/buildbot.git/+log/';

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

  private _helpUrl: string = 'http://go/perf-user-doc';

  private _reportBugUrl: string =
    'https://issuetracker.google.com/issues/new?component=1547614&template=1970127';

  private isHiddenTriage =
    window.perf.show_triage_link !== null ? window.perf.show_triage_link : true;

  constructor() {
    super(PerfScaffoldSk.template);
  }

  private static template = (ele: PerfScaffoldSk) => html`
  <app-sk>
    <header id=topbar>
      <div class="header-brand">
        <a href="/">
          <img class="logo" src="/dist/images/chrome-logo.svg?v=1" alt="Chrome" />
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
        ${this.revisionLinkTemplate()}
        <a href="${ele._helpUrl}" target="_blank" tab-index=0 >
          <help-icon-sk></help-icon-sk><span>Help</span>
        </a>
        <a href="${ele._reportBugUrl}" target="_blank" tab-index=0 >
          <bug-report-icon-sk></bug-report-icon-sk><span>Report Bug</span>
        </a>
        ${ele.buildTagTemplate()}
      </div>
      <div id=help>
      </div>
      <div id=chat>
      </div>
    </aside>
    <main>
    </main>
    <footer class="glue-footer">
      <error-toast-sk></error-toast-sk>
    </footer>
  </app-sk>
`;

  private static revisionLinkTemplate = () => {
    if (window.perf.fetch_chrome_perf_anomalies) {
      return html`<a href="/v" tab-index="0"
        ><trending-up-icon-sk></trending-up-icon-sk><span>Revision Info</span></a
      >`;
    }

    return html``;
  };

  private buildTagLinkTemplate(tag: string) {
    return html`<a href="${BUILDBOT_GIT}${tag}" target="_blank">Build: ${tag}</a>`;
  }

  private buildTagTemplate() {
    const buildTag = getBuildTag();
    return html`${choose(
      buildTag.type,
      [
        ['git', () => this.buildTagLinkTemplate(buildTag.tag!)],
        ['louhi', () => this.buildTagLinkTemplate(buildTag.tag!)],
        ['tag', () => html`<a>Build: ${buildTag.tag}</a>`],
      ],
      () => html`<a>Build: No Tag</a>`
    )}`;
  }

  private instanceTitleTemplate() {
    if (window.perf.instance_url) {
      return html`<a>${this.extractInstanceNameFromUrl(window.perf.instance_url)}</a>`;
    }
  }

  // Extract the string between "https://" and the first "." in a URL,
  // such as "https://androidx2-perf.luci.app"
  private extractInstanceNameFromUrl(url: string): string {
    const regex = /^https:\/\/(.*?)\./;
    const match = url.match(regex);

    // If a match is found, the captured group (the part between https:// and .)
    // will be at index 1 of the match array.
    if (match && match[1]) {
      return match[1].at(0)!.toUpperCase() + match[1].substring(1);
    }
    return '';
  }

  connectedCallback(): void {
    super.connectedCallback();
    // Don't call more than once.
    if (this._main) {
      return;
    }
    // We aren't using shadow dom so we need to manually move the children of
    // perf-scaffold-sk to be children of 'main'. We have to do this for the
    // existing elements and for all future mutations.

    // Create a temporary holding spot for elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Override the help url if specified in the instance config
    if (window.perf.help_url_override && window.perf.help_url_override !== '') {
      this._helpUrl = window.perf.help_url_override;
    }

    // Override the feedback / report bug url if specified in the instance config
    if (window.perf.feedback_url && window.perf.feedback_url !== '') {
      this._reportBugUrl = window.perf.feedback_url;
    }

    // Now that we've moved all the old children out of the way we can render
    // the template.
    this._render();

    this._main = this.querySelector('main');
    this._help = this.querySelector('#help');
    this._chat = this.querySelector('#chat');

    if (this._chat !== null) {
      this.addUrlToElement(this._chat, 'Ask the team', window.perf.chat_url);
    }

    // Move the old children back.
    this.redistributeAddedNodes(div.childNodes);

    // Move all future children also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        this.redistributeAddedNodes(mut.addedNodes);
      });
    });
    observer.observe(this, { childList: true });
  }

  // addUrlToElement adds the provided url data inside the given html element.
  private addUrlToElement(element: HTMLElement, urlText: string, urlHref: string) {
    element.hidden = true;
    if (urlHref && urlHref !== '') {
      element.innerHTML = `<a target="_blank" href="${urlHref}"
        tab-index=0><h4>${urlText}</h4></a>`;
      element.hidden = false;
    }
  }

  // Place these newly added nodes in the right place under the perf-scaffold-sk
  // element.
  private redistributeAddedNodes(from: NodeList) {
    Array.prototype.slice.call(from).forEach((node: Node) => {
      if ((node as Element).id === SIDEBAR_HELP_ID) {
        this._help!.appendChild(node);
      } else {
        this._main!.appendChild(node);
      }
    });
  }
}

define('perf-scaffold-sk', PerfScaffoldSk);

/** Root element for all codesize.skia.org pages. */

import { html, TemplateResult } from 'lit-html';

import { define } from 'elements-sk/define';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/spinner-sk';
import { END_BUSY_EVENT } from './events';

/** Moves the elements in a NodeList or HTMLCollection as children of another element. */
function move(from: HTMLCollection | NodeList, to: HTMLElement) {
  Array.prototype.slice.call(from).forEach((ele) => to.appendChild(ele));
}

export class CodesizeScaffoldSk extends ElementSk {
  private static template = (): TemplateResult => html`
    <app-sk>
      <header>
        <h1>Skia Code Size</h1>
        <spinner-sk></spinner-sk>
        <div class="spacer"></div>
        <theme-chooser-sk></theme-chooser-sk>
      </header>

      <aside>
        <ul>
          <li><a href="/">Home</a></li>
        </ul>
        <ul>
          <li><a href="https://chrome-supersize.firebaseapp.com/"
                 target="_blank" rel="noopener noreferrer"
                 title="View Chrome's codesize tool (called Super Size Tiger View) which has
                        input data from Chrome's apk for Android systems."
                 >Chrome's Tool</a></li>
        </ul>
      </aside>

      <main></main>

      <error-toast-sk></error-toast-sk>
    </app-sk>
  `;

  private main: HTMLElement | null = null;

  private busyTaskCount = 0;

  private spinner: SpinnerSk | null = null;

  constructor() {
    super(CodesizeScaffoldSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    // Don't call more than once.
    if (this.main) {
      return;
    }

    // We aren't using shadow dom so we need to manually move the children of codesize-scaffold-sk
    // to be children of 'main'. We have to do this for the existing elements and for all future
    // mutations.

    // Create a temporary holding spot for the elements we're moving.
    const div = document.createElement('div');
    move(this.children, div);

    // Now that we've moved all the old children out of the way we can render the template.
    this._render();

    // Move the old children back under main.
    this.main = this.querySelector('main');
    move(div.children, this.main!);

    // Move all future children under main also.
    const observer = new MutationObserver((mutList) => {
      mutList.forEach((mut) => {
        move(mut.addedNodes, this.main!);
      });
    });
    observer.observe(this, { childList: true });

    this.spinner = this.querySelector<SpinnerSk>('spinner-sk');

    // Any unhandled errors will be reported in the UI via an error toast.
    window.addEventListener('error', this.onUnhandledError);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener('error', this.onUnhandledError);
  }

  private onUnhandledError(e: ErrorEvent): void {
    errorMessage(e.error.message);
  }

  /**
   * Shows a loading indicator while awaiting the given promise and returns its fulfilled value. If
   * the promise is rejected, the rejection reason will be displayed in the UI as an error toast.
   *
   * This function can be called multiple times concurrently.
   */
  static waitFor<T>(promise: Promise<T>): Promise<T> {
    const scaffold = document.querySelector<CodesizeScaffoldSk>('codesize-scaffold-sk')!;
    return scaffold.waitFor(promise);
  }

  private async waitFor<T>(promise: Promise<T>): Promise<T> {
    this.busyTaskCount++;
    this.spinner!.active = true;
    try {
      return await promise;
    } catch (e) {
      await errorMessage(e);
      throw e;
    } finally {
      this.busyTaskCount--;
      if (this.busyTaskCount === 0) {
        this.spinner!.active = false;
        // Puppeteer tests can wait for this event before taking screenshots to ensure that the
        // page's contents are fully loaded.
        this.dispatchEvent(new CustomEvent(END_BUSY_EVENT, { bubbles: true }));
      }
    }
  }
}

define('codesize-scaffold-sk', CodesizeScaffoldSk);

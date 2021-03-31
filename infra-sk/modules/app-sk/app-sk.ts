/**
 * @module infra-sk/modules/app-sk
 * @description <h2><code>app-sk</code></h2>
 *
 * <p>
 *   A general application layout which includes a responsive
 *   side panel. This element is largely CSS, with a smattering of
 *   JS to toggle the side panel on/off when in small screen mode.
 *   This CSS has some variables which can be customized (see
 *   app-sk.scss and app-sk_demo.css for an example of this).
 * </p>
 * <p>
 *   In general, create an app with a &lt;header&gt;, aside, main, and footer
 *   and app-sk will apply the appropriate CSS to make an app with a
 *   header bar, a collapsible (on small screens) side-bar, the main content,
 *   and a footer.
 * </p>
 *
 * @example
 *  <app-sk>
 *    <header>
 *      <h1>Hello App</h1>
 *      <div class=spacer></div>
 *      <login-sk></login-sk>
 *    </header>
 *    <aside>
 *      <h2>This is the side bar</h2>
 *      <h2>Hopefully the side bar handles really wide text well.</h2>
 *    </aside>
 *
 *    <main>
 *      <h1>The main content!</h1>
 *    </main>
 *
 *    <footer><error-toast-sk></error-toast-sk></footer>
 *  </app-sk>
 *
 */
import { define } from 'elements-sk/define'
import 'elements-sk/icon/menu-icon-sk'

const buttonTemplate = document.createElement('template');
buttonTemplate.innerHTML = `
  <button class=toggle-button>
    <menu-icon-sk>
    </menu-icon-sk>
  </button>
`;

export class AppSk extends HTMLElement {
  connectedCallback() {
    const header = this.querySelector('header');
    const sidebar = this.querySelector('aside');
    if (!header || !sidebar) {
      return;
    }
    // Add the collapse button to the header as the first item.
    let btn = buttonTemplate.content.cloneNode(true) as HTMLButtonElement;
    // btn is a document-fragment, so we need to insert it into the
    // DOM to make it "expand" into a real button.
    header.insertBefore(btn, header.firstElementChild);
    btn = header.firstElementChild as HTMLButtonElement
    btn.addEventListener('click', () => {
      let sidebar = this.querySelector('aside')!;
      sidebar.classList.toggle('shown');
    });
  }
}

define('app-sk', AppSk);

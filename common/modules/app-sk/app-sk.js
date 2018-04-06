/**
 * @module common/app-sk
 * @description TODO(kjlubick)
 *
 * <p>
 * The <login-sk> custom element. Uses the Login promise to display the
 * current login status and provides login/logout links. Reports errors via
 * {@linkcode module:common/errorMessage}.
 * </p>
 */


window.customElements.define('app-sk', class extends HTMLElement {
  connectedCallback() {
    let header = this.querySelector('header');
    let sidebar = this.querySelector('aside');
    if (!header || !sidebar) {
      return;
    }
    // Add the collapse button to the header as the first item.
    let btn = document.createElement('button');
    btn.classList.add('toggle-button');
    let i = document.createElement('icon-menu-sk');
    btn.appendChild(i);
    btn.addEventListener('click', (e) => this._toggleMenu(e));
    header.insertBefore(btn, header.firstElementChild);
  }


  _toggleMenu(e) {
    let sidebar = this.querySelector('aside');
    sidebar.classList.toggle('shown');
  }
});

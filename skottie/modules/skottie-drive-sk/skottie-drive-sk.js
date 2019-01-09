/**
 * @module skottie-drive-sk
 * @description <h2><code>skottie-drive-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import '../skottie-player-sk'

let gapiLoaded = new Promise((resolve, reject) => {
  document.addEventListener('gapi-loaded', () => {
    resolve();
  })
});

window.customElements.define('skottie-drive-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this._render();
    gapiLoaded.then(() => {
       gapi.load('client:auth2', this.initClient);
    });
  }

  initClient() {
    gapi.client.init({
      apiKey: "",
      clientId: "",
      discoveryDocs: ["https://www.googleapis.com/discovery/v1/apis/drive/v3/rest"],
      scopes: ['https://www.googleapis.com/auth/drive.readonly', 'https://www.googleapis.com/auth/drive.install'],
    }).then(function () {
      let isSignedIn = gapi.auth2.getAuthInstance().isSignedIn;
      // Listen for sign-in state changes.
      isSignedIn.listen(this.updateSigninStatus);
      // Handle the initial sign-in state.
      this.updateSigninStatus(isSignedIn.get());
    });
  }

  /**
   *  Called when the signed in status changes, to update the UI
   *  appropriately. After a sign-in, the API is called.
   */
  updateSigninStatus(isSignedIn) {
  }

  disconnectedCallback() {
  }

  _render() {
    this.innerHTML = `<skottie-player-sk></skottie-player-sk>`;
  }

});

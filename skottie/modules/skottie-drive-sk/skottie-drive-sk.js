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
  let check = () => {
    if (window.gapi !== undefined) {
      resolve();
    } else {
      setTimeout(check, 10)
    }
  }
  setTimeout(check, 10)
});

window.customElements.define('skottie-drive-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    this._render();
    gapiLoaded.then(() => {
      gapi.load('client:auth2', () => { this.initClient() });
    });
  }

  initClient() {
    let doit = () => {
      console.log(this);
      let isSignedIn = gapi.auth2.getAuthInstance().isSignedIn;
      // Listen for sign-in state changes.
      isSignedIn.listen(this.updateSigninStatus);
      // Handle the initial sign-in state.
      this.updateSigninStatus(isSignedIn.get());
    };
    gapi.client.init({
      apiKey: 'AIzaSyD2US0bcYT2VhguMezYgDa4lbZc6rIQbDg', // API Key is locked to https://skottie.skia.org.
      clientId: '145247227042-fetft5vnkf582o817e1t553cm3tjvobl.apps.googleusercontent.com',
      discoveryDocs: ['https://www.googleapis.com/discovery/v1/apis/drive/v3/rest'],
      scope: 'https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/drive.install',
    }).then(doit);
  }

  /**
   *  Called when the signed in status changes, to update the UI
   *  appropriately. After a sign-in, the API is called.
   */
  updateSigninStatus(isSignedIn) {
    console.log(isSignedIn);
    if (!isSignedIn) {
      gapi.auth2.getAuthInstance().signIn();
    }
  }

  disconnectedCallback() {
  }

  _render() {
    this.innerHTML = `<skottie-player-sk></skottie-player-sk>`;
  }

});

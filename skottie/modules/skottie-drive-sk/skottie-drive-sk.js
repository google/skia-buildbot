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
import 'elements-sk/error-toast-sk'
import { $$ } from 'common-sk/modules/dom'
import { SKIA_VERSION } from '../../build/version.js'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { until } from 'lit-html/directives/until.js';

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

const caption = (id) => {
  /*
  return html`<pre>${id}</pre>`;
  */
  return gapiLoaded.then(() => {
    // Metadata.
    return gapi.client.drive.files.get({
      alt: 'json',
      fileId: id,
    }).then((response)  => {
      console.log(response.result.name);
      return html`<pre>${response.result.name}</pre>`;
    }).catch((response) => {
      errorMessage(response.result.error.message, 0);
    });
  });
}

const template = (ele) => html`
<header>
  <h2>Lottie Drive Previewer</h2><span><a href='https://skia.googlesource.com/skia/+/${SKIA_VERSION}'>${SKIA_VERSION.slice(0, 7)}</a></span>
</header>
<main>
  ${ele._ids.map((id, i) => html`<figure>
    <skottie-player-sk id="x${i}"></skottie-player-sk>
    <figcaption>${until(caption(id), `Loading...`)}</figcaption>
  </figure>`)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;


window.customElements.define('skottie-drive-sk', class extends HTMLElement {
  constructor() {
    super();
    this._ids = [];
  }

  connectedCallback() {
    this._render();
    gapiLoaded.then(() => {
      gapi.load('client:auth2', () => { this.initClient() });
    });
  }

  initClient() {
    let doit = () => {
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
    if (!isSignedIn) {
      gapi.auth2.getAuthInstance().signIn();
    } else {
      this.loadFiles();
    }
  }

  loadFiles() {
    // The state parameter is JSON that looks like:
    //
    // {
    //   "ids": ["0Bz0bd"],
    //   "action":"open",
    //   "userId":"103354693083460731603"
    // }
    //
    // See https://developers.google.com/drive/api/v3/integrate-open for more
    // details.
    let ids = ['12M0hlsK-zYCrKU6TG-Bji5Kcr9hWoyJw'];
    let stateParam = (new URL(document.location)).searchParams.get('state');
    if (stateParam) {
      ids = JSON.parse(stateParam).ids;
    }
    this._ids = ids;
    this._render();
    // Now kick off a fetch request for each player that retrieves the JSON
    // and populates the player.
    ids.forEach((id, i) => {
      // Media.
      gapi.client.drive.files.get({
        alt: 'media',
        fileId: id,
      }).then((response) => {
        if (response.headers['Content-Type'] !== 'application/json') {
          errorMessage("Can only process JSON files.", 0);
          return;
        }
        let lottie = response.result;
        let init = {
          width : lottie.w || 128,
          height : lottie.h || 128,
          lottie : lottie,
        }
        $$(`#x${i}`, this).initialize(init);
      }).catch((response) => {
        errorMessage(response.result.error.message, 0);
      });

    });

  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});

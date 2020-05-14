/**
 * @module skottie-drive-sk
 * @description <h2><code>skottie-drive-sk</code></h2>
 *
 *    Presents a page for previewing Lottie files that are coming
 *    from Google Drive.
 *
 *    All the work in the app is done in the browser, i.e. no server side
 *    work is done. Authentication is done via the Google JS client library
 *    and rendering of the Lottie files is done using the WASM version of
 *    Skia.
 *
 *    A link from Google Drive to preview a JSON file will include a query
 *    parameter named 'state' that looks like:
 *
 *    {
 *      "ids": ["0Bz0bd"],
 *      "action":"open",
 *      "userId":"103354693083460731603"
 *    }
 *
 *    Where 'ids' is a list of Google Drive document ids that can be used
 *    via the Google Drive API to retrieve the document or metadata about
 *    the document.
 *
 */
import '../skottie-player-sk'
import 'elements-sk/error-toast-sk'
import { $$ } from 'common-sk/modules/dom'
import { SKIA_VERSION } from '../../build/version.js'
import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { until } from 'lit-html/directives/until.js';

// gapiLoaded is a promise that resolves when the 'gapi' JS library is
// finished loading.
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

// caption returns a promise that resolves to the filename of the document
// with the given drive id.
const caption = (id) => {
  return gapiLoaded.then(() => {
    // Metadata.
    return gapi.client.drive.files.get({
      alt: 'json',
      fileId: id,
    }).then((response)  => {
      return html`<pre>${response.result.name}</pre>`;
    }).catch((response) => {
      errorMessage(response.result.error.message, 0);
    });
  });
}

// players returns one <skottie-player-sk> for each id.
const players = (ele) => ele._ids.map((id, i) => html`
<figure>
  <skottie-player-sk id="x${i}"></skottie-player-sk>
  <figcaption><p>${until(caption(id), `Loading...`)}</p><p class=errors id="errors${i}"></p></figcaption>
</figure>`);

const template = (ele) => html`
<header>
  <h2>Skia Lottie Drive Previewer</h2><span><a href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'>${SKIA_VERSION.slice(0, 7)}</a></span>
</header>
<main>
  ${players(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

define('skottie-drive-sk', class extends HTMLElement {
  constructor() {
    super();

    // The list of ids of documents to display.
    this._ids = [];
  }

  connectedCallback() {
    this._render();
    gapiLoaded.then(() => {
      // Load both the JS Apiary client library and OAuth 2 client library.
      gapi.load('client:auth2', () => { this.initClient() });
    });
  }

  initClient() {
    let postInit = () => {
      let isSignedIn = gapi.auth2.getAuthInstance().isSignedIn;
      // Listen for sign-in state changes.
      isSignedIn.listen(this.updateSigninStatus);
      // Handle the initial sign-in state.
      this.updateSigninStatus(isSignedIn.get());
    };

    gapi.client.init({
      apiKey: 'AIzaSyD2US0bcYT2VhguMezYgDa4lbZc6rIQbDg', // API Key is locked to https://skottie.skia.org, so it's safe to hardcode here.
      clientId: '145247227042-fetft5vnkf582o817e1t553cm3tjvobl.apps.googleusercontent.com', // Not protected info (clientSecret would be).
      discoveryDocs: ['https://www.googleapis.com/discovery/v1/apis/drive/v3/rest'],
      scope: 'https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/drive.install',
    }).then(postInit);
  }

  // updateSigninStatus is called when the signed in status changes.
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
    // Render a <skottie-player-sk> for each id.
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
          $$(`#errors${i}`, this).textContent = `Error: Not a JSON file.`;
          errorMessage("Can only process JSON files.", 0);
          return;
        }
        let lottie = response.result;
        let init = {
          width : lottie.w || 128,
          height : lottie.h || 128,
          lottie : lottie,
        }
        $$(`#x${i}`, this).initialize(init).catch((msg) => {
          $$(`#errors${i}`, this).textContent = `Error: Not a valid Lottie file.`;
          errorMessage(msg, 0);
        });
      }).catch((response) => {
        errorMessage(response.result.error.message, 0);
      });
    });
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});

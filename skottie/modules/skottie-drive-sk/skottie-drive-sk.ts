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
import '../skottie-player-sk';
import '../../../elements-sk/modules/error-toast-sk';
import { html } from 'lit/html.js';
import { until } from 'lit/directives/until.js';
import { $$ } from '../../../infra-sk/modules/dom';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  SkottiePlayerConfig,
  SkottiePlayerSk,
} from '../skottie-player-sk/skottie-player-sk';
import { LottieAnimation } from '../types';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

// See https://stackoverflow.com/a/45352250
interface WindowWithGAPILoaded extends Window {
  gapi: any;
}

declare let window: WindowWithGAPILoaded;

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

// gapiLoaded is a promise that resolves when the 'gapi' JS library is
// finished loading.
const gapiLoaded = new Promise<void>((resolve) => {
  const check = () => {
    if (window.gapi !== undefined) {
      resolve();
    } else {
      setTimeout(check, 10);
    }
  };
  setTimeout(check, 10);
});

interface ErrorResponse {
  result: {
    error: {
      message: string;
    };
  };
}

interface DriveGetResponse {
  headers: Record<string, string>;
  result: LottieAnimation;
}

interface URLState {
  ids: string[];
}

export class SkottieDriveSk extends ElementSk {
  private static template = (ele: SkottieDriveSk) => html`
    <app-sk>
      <header>
        <h2>Skia Lottie Drive Previewer</h2>
        <span>
          <a href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
            >${SKIA_VERSION.slice(0, 7)}</a
          >
          <theme-chooser-sk></theme-chooser-sk>
        </span>
      </header>
      <main>${ele.players()}</main>
      <footer>
        <error-toast-sk></error-toast-sk>
      </footer>
    </app-sk>
  `;

  // caption returns a promise that resolves to the filename of the document
  // with the given drive id.
  private static caption = (id: string) =>
    gapiLoaded.then(() =>
      window.gapi.client.drive.files
        .get({
          alt: 'json',
          fileId: id,
        })
        .then(
          (response: DriveGetResponse) =>
            html`<pre>${response.result.name}</pre>`
        )
        .catch((err: ErrorResponse) => {
          errorMessage(err.result.error.message, 0);
        })
    );

  // players returns one <skottie-player-sk> for each id.
  private players = () =>
    this.ids.map(
      (id, i) =>
        html` <figure>
          <skottie-player-sk id="x${i}"></skottie-player-sk>
          <figcaption>
            <p>${until(SkottieDriveSk.caption(id), 'Loading...')}</p>
            <p class="errors" id="errors${i}"></p>
          </figcaption>
        </figure>`
    );

  // The list of ids of documents to display.
  private ids: string[] = [];

  constructor() {
    super(SkottieDriveSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    gapiLoaded.then(() => {
      // Load both the JS Apiary client library and OAuth 2 client library.
      window.gapi.load('client:auth2', () => {
        this.initClient();
      });
    });
  }

  initClient(): void {
    const postInit = () => {
      const isSignedIn = window.gapi.auth2.getAuthInstance().isSignedIn;
      // Listen for sign-in state changes.
      isSignedIn.listen(this.updateSigninStatus);
      // Handle the initial sign-in state.
      this.updateSigninStatus(isSignedIn.get());
    };

    window.gapi.client
      .init({
        // eslint-disable-next-line no-useless-concat
        apiKey: 'AIzaSyD2US0bcYT2Vh' + 'guMezYgDa4lbZc6rIQbDg', // API Key is locked to https://skottie.skia.org, so it's safe to hardcode here.
        clientId:
          '145247227042-fetft5vnkf582o817e1t553cm3tjvobl.apps.googleusercontent.com', // Not protected info (clientSecret would be).
        discoveryDocs: [
          'https://www.googleapis.com/discovery/v1/apis/drive/v3/rest',
        ],
        scope:
          'https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/drive.install',
      })
      .then(postInit);
  }

  // updateSigninStatus is called when the signed in status changes.
  updateSigninStatus(isSignedIn: boolean): void {
    if (!isSignedIn) {
      window.gapi.auth2.getAuthInstance().signIn();
    } else {
      this.loadFiles();
    }
  }

  loadFiles(): void {
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
    const stateParam = new URL(document.location.href).searchParams.get(
      'state'
    );
    if (stateParam) {
      ids = (JSON.parse(stateParam) as URLState).ids;
    }
    // Render a <skottie-player-sk> for each id.
    this.ids = ids;
    this._render();

    // Now kick off a fetch request for each player that retrieves the JSON
    // and populates the player.
    ids.forEach((id: string, i: number) => {
      // Media.
      window.gapi.client.drive.files
        .get({
          alt: 'media',
          fileId: id,
        })
        .then((response: DriveGetResponse) => {
          if (response.headers['Content-Type'] !== 'application/json') {
            $$<HTMLParagraphElement>(`#errors${i}`, this)!.textContent =
              'Error: Not a JSON file.';
            errorMessage('Can only process JSON files.', 0);
            return;
          }
          const lottie = response.result;
          const init: SkottiePlayerConfig = {
            width: +lottie.w || 128,
            height: +lottie.h || 128,
            lottie: lottie,
            fps: 0,
          };
          $$<SkottiePlayerSk>(`#x${i}`, this)!
            .initialize(init)
            .catch((msg) => {
              $$<HTMLParagraphElement>(`#errors${i}`, this)!.textContent =
                'Error: Not a valid Lottie file.';
              errorMessage(msg, 0);
            });
        })
        .catch((err: ErrorResponse) => {
          errorMessage(err.result.error.message, 0);
        });
    });
  }
}

define('skottie-drive-sk', SkottieDriveSk);

/**
 * @module modules/team-drive-sk
 * @description <h2><code>team-drive-sk</code></h2>
 *
 * Lists all the files that match the query in the team drive.
 *
 * @attr query - The hashtag to search for.
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import 'elements-sk/spinner-sk'
import 'elements-sk/error-toast-sk'
import 'elements-sk/styles/buttons'

// TODO Fill this out from here: https://developers.google.com/drive/api/v3/mime-types
const mimeTypesToPath = {
    'application/vnd.google-apps.presentation': 'presentation',
    'application/vnd.google-apps.spreadsheet': 'spreadsheets',
    'application/vnd.google-apps.document': 'document',
    'application/vnd.google-apps.drawing': 'drawings',
};

function linkFromFile(file) {
    return `https://docs.google.com/${mimeTypesToPath[file.mimeType]}/d/${file.id}`
}

const template = (ele) => html`
  ${ele._isSignedIn() ? html`` : html`<button @click=${() => ele._signIn()}>Authorize</button>` }

  <spinner-sk ?active=${ele._loading}></spinner-sk>
  <ul>
${ele._files.map((file) => html`<li><a href='${linkFromFile(file)}'>${file.name}</a></li>`)}
  </ul>
`;

// Client ID and API key from the Developer Console
const CLIENT_ID = '145247227042-9dc658m0dj9mbah94kn5i3nnejirvrgl.apps.googleusercontent.com';
const API_KEY = 'AIzaSyB8YZ_bFpEVOyICD-pUutJ3mxm6c1to8q0';

// Array of API discovery doc URLs for APIs used by the quickstart
const DISCOVERY_DOCS = ["https://www.googleapis.com/discovery/v1/apis/drive/v3/rest"];

// Authorization scopes required by the API; multiple scopes can be included,
// separated by spaces.
const SCOPES = 'https://www.googleapis.com/auth/drive.readonly';

// The Google Team Drive ID.
const DRIVE_ID = '0AOGploz136NUUk9PVA';

window.customElements.define('team-drive-sk', class extends ElementSk {
    constructor() {
        super(template);
        this._files = [];
        this._loading = true;
    }

    connectedCallback() {
        super.connectedCallback();
        this._render();
        gapi.load('client:auth2', () => this._initClient());
    }

    _initClient() {
        gapi.client.init({
            apiKey: API_KEY,
            clientId: CLIENT_ID,
            discoveryDocs: DISCOVERY_DOCS,
            scope: SCOPES
        }).then(() => {
            // Listen for sign-in state changes.
            gapi.auth2.getAuthInstance().isSignedIn.listen(() => this._listFiles());

            // Handle the initial sign-in state.
            this._listFiles();
        }).catch(errorMessage);
    }

    _signIn() {
        gapi.auth2.getAuthInstance().signIn();
    }

    _isSignedIn() {
        if (!gapi.auth2) {
            return true;
        }
        return gapi.auth2.getAuthInstance().isSignedIn.get();
    }

    _listFiles() {
        if (this._isSignedIn()) {
            this._files = [];
            gapi.client.drive.files.list({
                'pageSize': 10,
                'corpora': 'drive',
                'driveId': DRIVE_ID,
                'includeItemsFromAllDrives': true,
                // TODO Sanitize the value of the query attribute.
                'q': `fullText contains \"${this.getAttribute('query')}\"`,
                'supportsAllDrives': true,
                // TODO(jcgregorio): iterate the pages of the response.
                //'fields': "nextPageToken, files(id, name)"
            }).then((response) => {
                var files = response.result.files;
                if (files && files.length > 0) {
                    for (var i = 0; i < files.length; i++) {
                        this._files.push(files[i]);
                    }
                } else {
                    // None found.
                }
                this._loading = false;
                this._render();
            }).catch(errorMessage);
        } else {
            this._render();
        }
    }
});
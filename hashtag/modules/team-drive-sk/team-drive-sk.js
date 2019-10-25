/**
 * @module modules/team-drive-sk
 * @description <h2><code>team-drive-sk</code></h2>
 *
 * Lists all the files that match the query in the team drive.
 *
 * @attr query - The hashtag to search for.
 *
 * @attr client_id - The OAuth 2.0 Client ID.
 *
 * @attr api_key - The API Key.
 *
 * @attr drive_id - The Google Teams Drive ID.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import 'elements-sk/spinner-sk'
import 'elements-sk/error-toast-sk'
import 'elements-sk/styles/buttons'

// Full list of mime-types: https://developers.google.com/drive/api/v3/mime-types
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
  ${ele._isSignedIn() ? html`` : html`<button @click=${() => ele._signIn()}>Authorize</button>`}
  <spinner-sk ?active=${ele._loading}></spinner-sk>
  <ul>
    ${(!ele._loading && ele._files.length === 0) ? html`<p>None found.</p>` : html``}
    ${ele._files.map((file) => html`<li><a href='${linkFromFile(file)}'>${file.name}</a></li>`)}
  </ul>
`;

// Array of API discovery doc URLs for APIs used by the quickstart
const DISCOVERY_DOCS = ["https://www.googleapis.com/discovery/v1/apis/drive/v3/rest"];

// Authorization scopes required by the API; multiple scopes can be included,
// separated by spaces.
const SCOPES = 'https://www.googleapis.com/auth/drive.readonly';

class TeamDriveSk extends ElementSk {
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
            // Client ID and API key from the Developer Console
            apiKey: this.getAttribute("api_key"),
            clientId:  this.getAttribute("client_id"),
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
        // Protect against being called before the gapi library is loaded.
        // Default to true which will hide the login button and this avoids a
        // flash of the button if the lib is slow to load but the user is logged
        // in.
        if (!gapi.auth2) {
            return true;
        }
        return gapi.auth2.getAuthInstance().isSignedIn.get();
    }

    _listFiles() {
        if (this._isSignedIn()) {
            this._files = [];

            // TODO(jcgregorio): iterate the pages of the response.
            //'fields': "nextPageToken, files(id, name)"
            gapi.client.drive.files.list({
                'pageSize': 30,
                'corpora': 'drive',
                // The Google Team Drive ID.
                'driveId': this.getAttribute("drive_id"),
                'includeItemsFromAllDrives': true,
                'q': `fullText contains \"${this.getAttribute('query')}\"`,
                'supportsAllDrives': true,
            }).then((response) => {
                var files = response.result.files;
                if (files && files.length > 0) {
                    this._files = files;
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
}

// This element requires the Google JS API Client to be loaded, so don't
// register the element until the library is loaded.
if (window.gapi) {
    window.customElements.define('team-drive-sk', TeamDriveSk);
} else {
    // Look in index.html for the code that generates the event.
    document.addEventListener('gapi-loaded', () => {
        window.customElements.define('team-drive-sk', TeamDriveSk);
    });
}

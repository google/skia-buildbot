import { LitElement, TemplateResult, css, html } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { formatBug } from '../common/anomaly';
import '../window/window';
import { errorMessage } from '../errorMessage';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';
import { GetUserIssuesForTraceKeysResponse, UserIssue } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

@customElement('user-issue-sk')
export class UserIssueSk extends LitElement {
  static styles = css`
    .showLinkContainer {
      display: flex;
      align-items: center;

      .label {
        text-decoration: underline;
      }

      .linkContainer {
        color: var(--primary);
        font-size: 14px;
        display: flex;
        align-items: center;

        a {
          color: var(--primary);
          margin: 0 8px;
        }

        close-icon-sk {
          fill: var(--negative);
          cursor: pointer;
          height: 24px;
          width: 24px;
        }
      }
    }

    .new-issue-text-container {
      display: flex;
      align-items: center;
      padding-top: 25px;
      flex-direction: column;

      .new-issue {
        input {
          color: var(--on-surface);
          background: var(--surface);
          border: solid 1px var(--on-surface);
        }

        span {
          margin-left: 12px;

          check-icon-sk {
            fill: var(--positive);
            cursor: pointer;
            height: 24px;
            width: 24px;
          }

          close-icon-sk {
            fill: var(--negative);
            cursor: pointer;
            height: 24px;
            width: 24px;
          }
        }
      }
    }

    .new-issue-label {
      background: none;
      background: none !important;
      border: none;
      padding: 0 !important;
      color: var(--primary);
      text-decoration: underline;
      cursor: pointer;
      font-size: 14px;
    }

    .add-issue {
      align-items: center;
      background: transparent;
      border-radius: 4px;
      border: solid 1px var(--outline);
      box-shadow: none;
      color: var(--primary);
      display: inline-flex;
      fill: var(--primary);
      font-size: 11px;
      height: 24px;
      justify-content: center;
      margin: 0 2px;
      min-width: auto;
      padding: 0 4px;
      text-align: center;
      text-transform: none;
      white-space: nowrap;
      cursor: pointer;

      &:hover {
        background: var(--surface-1dp);
      }

      .icon-sk {
        font-size: 20px;
      }

      svg {
        width: 20px;
        height: 20px;
      }
    }

    .add-issue-container {
      display: flex;
      flex-direction: row;
      flex-wrap: nowrap;
      align-items: center;
      gap: 4px;
    }
  `;

  // Email of the logged in user. Empty string otherwise
  @property({ attribute: true })
  user_id: string = '';

  // bug_id = 0 signifies no buganizer issue available in the database for the
  // data point. bug_id > 0 means we have an existing buganizer issue.
  @property({ attribute: true })
  bug_id: number = 0;

  // The trace_key of the selected data point. Format: ,a=1,b=2,c=3,
  @property({ attribute: true })
  trace_key: string = '';

  // Commit position of the data point
  @property({ attribute: true })
  commit_position: number = 0;

  @property({ state: true })
  issueExists = false;

  // Used for capturing number input values
  _input_val: number = 0;

  // Indicates if the text input is active typically when adding a new issue.
  @property({ attribute: false })
  _text_input_active: boolean = false;

  @query('#loading-popup')
  private _loadingPopup!: HTMLDialogElement;

  connectedCallback() {
    super.connectedCallback();
    if (this.user_id !== '') {
      LoggedIn().then((status: LoginStatus) => {
        this.user_id = status.email;
      });
    }
  }

  render() {
    if (this.bug_id === -1) {
      return html``;
    }
    const template =
      this.issueExists && this.user_id !== '' ? this.showLinkTemplate() : this.addIssueTemplate();
    if (this.user_id === '') {
      return template;
    }
    return html`
      ${template}
      <dialog id="loading-popup">
        <p>Creating a new bug. Waiting...</p>
      </dialog>
    `;
  }

  // If a bug is already associated with the data point show them the link.
  // The delete action for this bug will only be shown if the user is logged in.
  showLinkTemplate(): TemplateResult {
    if (this.user_id === '') {
      return html`
        <div class="showLinkContainer">
          <span class="label">Bug:</span>
          <span class="linkContainer"> ${formatBug(window.perf.bug_host_url, this.bug_id)} </span>
        </div>
      `;
    }

    return html`
      <div class="showLinkContainer">
        <span class="label">Bug:</span>
        <span class="linkContainer">
          ${formatBug(window.perf.bug_host_url, this.bug_id)}
          <close-icon-sk @click=${this.removeIssue} ?hidden=${this.bug_id === 0}> </close-icon-sk>
        </span>
      </div>
    `;
  }

  private changeHandler(e: InputEvent) {
    this._input_val = +(e.target! as HTMLInputElement).value;
  }

  private activateTextInput() {
    this._text_input_active = true;
    this.render();
  }

  hideTextInput() {
    this._input_val = 0;
    this._text_input_active = false;
    this.render();
  }

  /* Templates */

  // Template for showing option to add an issue on the datapoint
  // Only shown when the user is logged in
  addIssueTemplate(): TemplateResult {
    if (this.user_id === '') {
      return html``;
    }

    if (this._text_input_active) {
      return html`
        <div class="new-issue-text-container">
          <span class="new-issue">
            <input
              style="width: 100px;"
              placeholder="eg: 3368155"
              type="number"
              min="0"
              @input=${this.changeHandler} />
          </span>
          <span>
            <check-icon-sk
              id="check-icon"
              @click=${() => {
                this.findOrAddIssue();
              }}></check-icon-sk>
            <close-icon-sk @click=${this.hideTextInput}></close-icon-sk>
          </span>
        </div>
      `;
    }

    return html`
      <div class="add-issue-container">
        <button class="add-issue" @click=${this.activateTextInput}>Add Existing Bug</button>
        <button class="add-issue" @click=${this.createNewBug}>Add New Bug</button>
      </div>
    `;
  }

  /* API CALLS */
  // Makes an api call to create a new buganizer issue if the bug not exists for the trace key
  private async createNewBug() {
    this._loadingPopup.showModal();
    const body = {
      trace_names: [this.trace_key],
      commit_position: this.commit_position,
    };

    try {
      const resp = await fetch('/_/user_issue/create', {
        method: 'POST',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const json = await jsonOrThrow(resp);
      this._loadingPopup.close();

      // Open the bug page in new window.
      const bugUrl = `https://issues.chromium.org/issues/${json.bug_id}`;
      window.open(bugUrl, '_blank');

      this.bug_id = json.bug_id;
      this.issueExists = true;

      this.dispatchEvent(
        new CustomEvent('user-issue-changed', {
          detail: {
            trace_key: this.trace_key,
            commit_position: this.commit_position,
            bug_id: this.bug_id,
          },
          bubbles: true,
        })
      );
    } catch (e) {
      this._loadingPopup.close();
      errorMessage(
        'File new bug request failed due to an internal server error. Please try again.'
      );
      throw e;
    }
  }

  // Makes an api call to delete a buganizer issue
  // Also emits an event to refresh the existing list of user issues
  // with the newly deleted object.
  private async removeIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;

    const req = {
      trace_key: traceKey,
      commit_position: commitPosition,
    };
    const resp = await fetch('/_/user_issue/delete', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!resp.ok) {
      const msg = await resp.text();
      errorMessage(`${resp.statusText}: ${msg}`);
      return;
    }

    // Since the bug is deleted now, we set the bug_id to 0 for showing the
    // Add buganizer issue template
    this.bug_id = 0;
    this.issueExists = false;
    this._input_val = 0;
    this._text_input_active = false;

    this.dispatchEvent(
      new CustomEvent('user-issue-changed', {
        detail: {
          trace_key: this.trace_key,
          commit_position: this.commit_position,
          bug_id: this.bug_id,
        },
        bubbles: true,
      })
    );
  }

  private async findOrAddIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;

    //check if the issue already exists in the database. If it does not exist, create a
    // new bug issue first, then add it to the bug issue list for the trace key
    const getIssueRequest = {
      trace_keys: [traceKey],
      begin_commit_position: commitPosition,
      end_commit_position: commitPosition,
    };
    try {
      const json: GetUserIssuesForTraceKeysResponse = await fetch('/_/user_issues', {
        method: 'POST',
        body: JSON.stringify(getIssueRequest),
        headers: {
          'Content-Type': 'application/json',
        },
      }).then(jsonOrThrow);

      const userIssues: UserIssue[] = json.UserIssues || [];
      let issueFound = false;
      if (userIssues.length !== 0) {
        userIssues.forEach((userIssue) => {
          if (userIssue.IssueId === this._input_val) {
            this.bug_id = this._input_val;
            issueFound = true;
          }
        });
      }

      if (issueFound) {
        this.issueExists = true;
      } else {
        await this.createNewBug();
        this.issueExists = true;
      }
    } catch (json) {
      const msg = await (json as Response).text();
      errorMessage(`${(json as Response).statusText}: ${msg}`);
    }
  }
}

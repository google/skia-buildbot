import { LitElement, TemplateResult, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { errorMessage } from '../errorMessage';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';

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

    .new-issue-text-container {
      display: flex;
      align-items: center;

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

    .new-bug {
      align-items: center;
      background: transparent;
      border-radius: 4px;
      border: solid 1px var(--outline);
      box-shadow: none;
      color: var(--primary);
      display: inline-flex;
      fill: var(--primary);
      font-size: 14px;
      height: 24px;
      justify-content: center;
      margin: 4px 2px;
      min-width: auto;
      padding: 0 4px;
      text-align: center;
      text-transform: none;

      .icon-sk {
        font-size: 20px;
      }

      svg {
        width: 20px;
        height: 20px;
      }
    }
  }
  `;

  // Email of the logged in user. Empty string otherwise
  _user_id: string = '';

  // Used for capturing number input values
  _input_val: number = 0;

  // Indicates if the text input is active typically when adding a new issue.
  @property({ attribute: false })
  _text_input_active: boolean = false;

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

  connectedCallback() {
    super.connectedCallback();

    LoggedIn().then((status: LoginStatus) => {
      this._user_id = status.email;
    });
  }

  render() {
    if (this.bug_id === -1) {
      return html``;
    }
    return html`${this.bug_id === 0 ? this.addIssueTemplate() : this.showLinkTemplate()}`;
  }

  /* Template manipulation functions */
  private activateTextInput() {
    this._text_input_active = true;
    this.render();
  }

  private changeHandler(e: InputEvent) {
    this._input_val = +(e.target! as HTMLInputElement).value;
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
    if (this._user_id === '') {
      return html``;
    }

    if (this._text_input_active) {
      return html`
        <div class="new-issue-text-container">
          <input placeholder="eg: 91345" type="number" min="0" @input=${this.changeHandler} />
          <span>
            <check-icon-sk
              @click=${() => {
                this.addIssue();
              }}></check-icon-sk>
            <close-icon-sk @click=${this.hideTextInput}></close-icon-sk>
          </span>
        </div>
      `;
    }

    return html`
      <div>
        <button class="new-bug" @click=${this.activateTextInput}>Add Bug#</button>
      </div>
    `;
  }

  // If a bug is already associated with the data point show them the link.
  // The delete action for this bug will only be shown if the user is logged in.
  showLinkTemplate(): TemplateResult {
    if (this._user_id === '') {
      return html`
        <div class="showLinkContainer">
          <span class="label"> User Thread: </span>
          <span class="linkContainer">
            ${AnomalySk.formatBug(window.perf.bug_host_url, this.bug_id)}
          </span>
        </div>
      `;
    }

    return html`
      <div class="showLinkContainer">
        <span class="label"> User Thread: </span>
        <span class="linkContainer">
          ${AnomalySk.formatBug(window.perf.bug_host_url, this.bug_id)}
          <close-icon-sk @click=${this.removeIssue} ?hidden=${this.bug_id === 0}> </close-icon-sk>
        </span>
      </div>
    `;
  }

  /* API CALLS */

  // Makes an api call to save a buganizer issue
  // Also emits an event to refresh the existing list of user issues
  // with the newly added object.
  private async addIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;

    if (this._input_val === 0) {
      errorMessage(`Input a valid bug id. For example, 34243`);
      return;
    }

    const req = {
      trace_key: traceKey,
      commit_position: commitPosition,
      issue_id: this._input_val,
    };
    const resp = await fetch('/_/user_issue/save', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!resp.ok) {
      this.hideTextInput();
      const msg = await resp.text();
      errorMessage(`${resp.statusText}: ${msg}`);
      return;
    }

    this.bug_id = this._input_val;
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

    this.render();
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

    this.render();
  }
}

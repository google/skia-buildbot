import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/send-icon-sk';
import '../../../elements-sk/modules/spinner-sk';

@customElement('gemini-side-panel-sk')
export class GeminiSidePanelSk extends LitElement {
  @state()
  private messages: { role: 'user' | 'model'; text: string }[] = [];

  @state()
  private isLoading: boolean = false;

  @state()
  private input: string = '';

  @property({ type: Boolean, reflect: true })
  open: boolean = false;

  constructor() {
    super();
  }

  toggle() {
    this.open = !this.open;
  }

  private onInput(e: Event) {
    this.input = (e.target as HTMLInputElement).value;
  }

  private async send() {
    if (!this.input.trim()) return;
    const userMsg = this.input;
    this.messages = [...this.messages, { role: 'user', text: userMsg }];
    this.input = '';
    this.isLoading = true;

    try {
      const response = await fetch('/_/chat', {
        method: 'POST',
        body: JSON.stringify({ query: userMsg }),
        headers: { 'Content-Type': 'application/json' },
      });
      if (response.ok) {
        const json = await response.json();
        this.messages = [...this.messages, { role: 'model', text: json.response || 'No response' }];
      } else {
        this.messages = [
          ...this.messages,
          { role: 'model', text: 'Error: ' + response.statusText },
        ];
      }
    } catch (_e) {
      this.messages = [...this.messages, { role: 'model', text: 'Error sending message.' }];
    } finally {
      this.isLoading = false;
    }
  }

  static styles = css`
    :host {
      display: block;
      position: fixed;
      top: 0;
      right: -400px;
      width: 400px;
      height: 100%;
      background: var(--background-color, white);
      box-shadow: -2px 0 5px rgba(0, 0, 0, 0.2);
      transition: right 0.3s ease-in-out;
      z-index: 1000;
      display: flex;
      flex-direction: column;
    }

    :host([open]) {
      right: 0;
    }

    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px;
      background: var(--primary-color, #1976d2);
      color: white;
    }

    #chat-history {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .message {
      padding: 8px 12px;
      border-radius: 8px;
      max-width: 80%;
      word-wrap: break-word;
    }

    .user {
      align-self: flex-end;
      background: #e3f2fd;
      color: black;
    }

    .model {
      align-self: flex-start;
      background: #f5f5f5;
      color: black;
    }

    footer {
      padding: 16px;
      display: flex;
      gap: 8px;
      border-top: 1px solid #ddd;
    }

    input {
      flex: 1;
      padding: 8px;
    }

    close-icon-sk,
    send-icon-sk {
      cursor: pointer;
      fill: currentColor;
    }
  `;

  render() {
    return html`
      <header>
        <h3>Gemini Assistant</h3>
        <close-icon-sk
          @click=${this.toggle}
          aria-label="Close panel"
          role="button"
          tabindex="0"></close-icon-sk>
      </header>
      <div id="chat-history" role="log" aria-live="polite">
        ${this.messages.map((msg) => html`<div class="message ${msg.role}">${msg.text}</div>`)}
        ${this.isLoading ? html`<spinner-sk></spinner-sk>` : ''}
      </div>
      <footer>
        <input
          type="text"
          .value=${this.input}
          @input=${this.onInput}
          @keydown=${(e: KeyboardEvent) => e.key === 'Enter' && this.send()}
          placeholder="Ask something..."
          aria-label="Chat input" />
        <send-icon-sk
          @click=${this.send}
          aria-label="Send message"
          role="button"
          tabindex="0"></send-icon-sk>
      </footer>
    `;
  }
}

import { html, css, LitElement } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { marked } from 'marked';
import '@material/web/button/filled-button.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/outlined-icon-button.js';
import '@material/web/progress/circular-progress.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

// Interface definitions based on the proto
export interface Topic {
  topicId: number;
  topicName: string;
  summary: string;
}

interface TopicDetails {
  topicId: number;
  topicName: string;
  summary: string;
  codeChunks: string[];
}

interface GetTopicsResponse {
  topics: Topic[];
}

interface GetTopicDetailsResponse {
  topics: TopicDetails[];
}

@customElement('rag-app-sk')
export class RagAppSk extends LitElement {
  @state() private query = '';

  @state() private topics: Topic[] = [];

  @state() private selectedTopic: TopicDetails | null = null;

  @state() private isLoading = false;

  @state() private topicCount = 10;

  @state() private instanceName = '';

  @state() private headerIconUrl = '';

  static styles = css`
    :host {
      display: block;
      height: 100vh;
      display: flex;
      flex-direction: column;
      padding: 16px;
      box-sizing: border-box;
      font-family: Roboto, sans-serif;
    }

    .title-bar {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 16px;
    }

    .title-bar img {
      width: 32px;
      height: 32px;
    }

    .title-bar h1 {
      margin: 0;
      font-size: 24px;
      font-weight: 500;
      color: #202124;
    }

    header {
      display: flex;
      gap: 16px;
      margin-bottom: 16px;
      align-items: center;
    }

    md-outlined-text-field.query-input {
      flex-grow: 1;
    }

    .counter-wrapper {
      display: flex;
      align-items: center;
      gap: 8px;
      white-space: nowrap;
    }

    .counter {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    md-outlined-text-field.count-input {
      width: 70px;
      --md-outlined-text-field-container-height: 40px;
      --md-outlined-text-field-top-space: 8px;
      --md-outlined-text-field-bottom-space: 8px;
    }

    md-outlined-text-field.count-input::part(input) {
      text-align: center;
    }

    md-outlined-icon-button {
      --md-outlined-icon-button-container-size: 40px;
      --md-outlined-icon-button-icon-size: 20px;
    }

    main {
      display: flex;
      flex-grow: 1;
      gap: 16px;
      overflow: hidden;
      border: 1px solid #ccc;
    }

    #topic-list {
      width: 30%;
      border-right: 1px solid #ccc;
      overflow-y: auto;
      padding: 8px;
    }

    #topic-details {
      width: 70%;
      overflow-y: auto;
      padding: 16px;
    }

    .topic-item {
      padding: 12px;
      border-bottom: 1px solid #eee;
      cursor: pointer;
    }

    .topic-item:hover {
      background-color: #f5f5f5;
    }

    .topic-item.selected {
      background-color: #e0e0e0;
      font-weight: bold;
    }

    .topic-name {
      font-size: 1.1em;
      margin-bottom: 4px;
    }

    .topic-summary {
      font-size: 0.9em;
      color: #666;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }

    pre {
      background-color: #f4f4f4;
      padding: 12px;
      border-radius: 4px;
      overflow-x: auto;
    }

    .spinner-overlay {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      background-color: rgba(255, 255, 255, 0.7);
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      z-index: 1000;
      gap: 16px;
    }

    .spinner-overlay span {
      font-size: 1.5em;
      font-weight: bold;
      color: #333;
    }
  `;

  private async search() {
    if (!this.query.trim()) return;

    this.isLoading = true;
    this.selectedTopic = null;
    this.topics = [];

    try {
      const resp = (await fetch(
        `/historyrag/v1/topics?query=${encodeURIComponent(this.query)}&topic_count=${
          this.topicCount
        }`
      ).then(jsonOrThrow)) as GetTopicsResponse;
      this.topics = resp.topics || [];
    } catch (error) {
      console.error(error);
      // Optionally show toast
    } finally {
      this.isLoading = false;
    }
  }

  private async selectTopic(topicId: number) {
    this.isLoading = true;
    try {
      const resp = (await fetch(
        `/historyrag/v1/topic_details?topic_ids=${topicId}&include_code=true`
      ).then(jsonOrThrow)) as GetTopicDetailsResponse;

      if (resp.topics && resp.topics.length > 0) {
        this.selectedTopic = resp.topics[0];
      }
    } catch (error) {
      console.error(error);
    } finally {
      this.isLoading = false;
    }
  }

  async connectedCallback() {
    super.connectedCallback();
    try {
      const config = (await fetch('/config').then(jsonOrThrow)) as {
        instance_name: string;
        header_icon_url: string;
      };
      this.instanceName = config.instance_name;
      this.headerIconUrl = config.header_icon_url;
    } catch (error) {
      console.error('Failed to fetch config', error);
    }
  }

  render() {
    return html`
      ${this.isLoading
        ? html`
            <div class="spinner-overlay">
              <md-circular-progress indeterminate></md-circular-progress>
              <span>Loading...</span>
            </div>
          `
        : ''}
      <div class="title-bar">
        ${this.headerIconUrl ? html`<img src="${this.headerIconUrl}" alt="Logo" />` : ''}
        <h1>${this.instanceName}</h1>
      </div>
      <header>
        <md-outlined-text-field
          class="query-input"
          label="Search topics..."
          .value=${this.query}
          @input=${(e: InputEvent) => (this.query = (e.target as HTMLInputElement).value)}
          @keydown=${(e: KeyboardEvent) =>
            e.key === 'Enter' && this.search()}></md-outlined-text-field>

        <div class="counter-wrapper">
          <span>Topic Count:</span>
          <div class="counter">
            <md-outlined-text-field
              class="count-input"
              type="number"
              .value=${this.topicCount.toString()}
              @input=${(e: InputEvent) =>
                (this.topicCount =
                  parseInt((e.target as HTMLInputElement).value) || 1)}></md-outlined-text-field>
          </div>
        </div>
        <md-filled-button @click=${this.search} ?disabled=${this.isLoading}
          >Search</md-filled-button
        >
      </header>

      <main>
        <div id="topic-list">
          ${this.topics.length > 0
            ? html`
                <div
                  style="padding: 12px; font-weight: bold; border-bottom: 1px solid #ccc; background-color: #f9f9f9;">
                  ${this.topics.length} matching topics
                </div>
              `
            : ''}
          ${this.topics.length === 0 && !this.isLoading
            ? html`<div style="padding:16px; color:#666">No topics found.</div>`
            : ''}
          ${this.topics.map(
            (topic) => html`
              <div
                class="topic-item ${this.selectedTopic?.topicId === topic.topicId
                  ? 'selected'
                  : ''}"
                @click=${() => this.selectTopic(topic.topicId)}>
                <div class="topic-name">${topic.topicName}</div>
                <div class="topic-summary">${unsafeHTML(marked.parse(topic.summary))}</div>
              </div>
            `
          )}
        </div>

        <div id="topic-details">
          ${this.selectedTopic
            ? html`
                <h1>${this.selectedTopic.topicName}</h1>
                <div class="topic-summary-full">
                  ${unsafeHTML(marked.parse(this.selectedTopic.summary))}
                </div>

                <h3>Code Chunks</h3>
                ${this.selectedTopic.codeChunks?.map((chunk) => html` <pre>${chunk}</pre> `)}
              `
            : html`
                <div
                  style="display:flex; height:100%; align-items:center; justify-content:center; color:#888;">
                  Select a topic to view details
                </div>
              `}
        </div>
      </main>
    `;
  }
}

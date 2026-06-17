/**
 * @module modules/triage-panel-sk
 * @description <h2><code>triage-panel-sk</code></h2>
 *
 * Triage Panel provides a staging area for performance sheriffs to create arbitrary custom buckets,
 * place anomalies into them, and perform mass triage actions.
 * It delegates individual bucket rendering and smart state machine handling (NEW -> APPLIED -> DIRTY)
 * to the child triage-bucket-sk component.
 * It features a drag-to-move header and a minimize/collapse toggle to prevent toolbar overlap.
 */
import { html, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import { Anomaly } from '../json';
import { TriageBucketsController, TriageBucket } from './buckets-controller';

import './triage-bucket-sk';
import '../../../elements-sk/modules/icons/add-icon-sk';
import '../../../elements-sk/modules/icons/content-copy-icon-sk';
import '../../../elements-sk/modules/icons/content-paste-icon-sk';

export { TriageBucket };

@customElement('triage-panel-sk')
export class TriagePanelSk extends LitElement {
  @property({ attribute: false })
  anomalies: Anomaly[] = [];

  @property({ attribute: false })
  traceNames: string[] = [];

  @state()
  newBucketName: string = '';

  @property({ type: Boolean, reflect: true })
  collapsed: boolean = false;

  private bucketsController = new TriageBucketsController(this);

  get buckets(): TriageBucket[] {
    return this.bucketsController.buckets;
  }

  private isDragging = false;

  private xOffset = 0;

  private yOffset = 0;

  private isResizing = false;

  private resizeDir: string = '';

  private startX = 0;

  private startY = 0;

  private startWidth = 0;

  private startHeight = 0;

  private startLeft = 0;

  private startTop = 0;

  createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    this.addEventListener('bucket-updated', this.onBucketUpdated as EventListener);
    this.addEventListener('bucket-deleted', this.onBucketDeleted as EventListener);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.removeEventListener('bucket-updated', this.onBucketUpdated as EventListener);
    this.removeEventListener('bucket-deleted', this.onBucketDeleted as EventListener);
    document.removeEventListener('mousemove', this.onMouseMove);
    document.removeEventListener('mouseup', this.onMouseUp);
  }

  private onMouseDown = (e: MouseEvent) => {
    const header = this.querySelector('.header');
    if (!header || !header.contains(e.target as Node)) {
      return;
    }
    document.addEventListener('mousemove', this.onMouseMove);
    document.addEventListener('mouseup', this.onMouseUp);
    this.isDragging = true;
    const rect = this.getBoundingClientRect();
    this.xOffset = e.clientX - rect.left;
    this.yOffset = e.clientY - rect.top;
  };

  private onMouseMove = (e: MouseEvent) => {
    if (!this.isDragging) return;
    e.preventDefault();
    const newLeft = e.clientX - this.xOffset;
    const newTop = e.clientY - this.yOffset;

    const maxLeft = window.innerWidth - this.offsetWidth;
    const maxTop = window.innerHeight - this.offsetHeight;

    this.style.left = `${Math.max(0, Math.min(newLeft, maxLeft))}px`;
    this.style.top = `${Math.max(0, Math.min(newTop, maxTop))}px`;
    this.style.right = 'auto';
  };

  private onMouseUp = (e: MouseEvent) => {
    if (this.isDragging) {
      e.preventDefault();
      this.isDragging = false;
      document.removeEventListener('mousemove', this.onMouseMove);
      document.removeEventListener('mouseup', this.onMouseUp);
    }
  };

  private onResizeMouseDown(e: MouseEvent, dir: string) {
    e.stopPropagation();
    e.preventDefault();
    this.isResizing = true;
    this.resizeDir = dir;
    this.startX = e.clientX;
    this.startY = e.clientY;
    const rect = this.getBoundingClientRect();
    this.startWidth = rect.width;
    this.startHeight = rect.height;
    this.startLeft = rect.left;
    this.startTop = rect.top;

    document.addEventListener('mousemove', this.onResizeMouseMove);
    document.addEventListener('mouseup', this.onResizeMouseUp);
  }

  private onResizeMouseMove = (e: MouseEvent) => {
    if (!this.isResizing) return;
    e.preventDefault();
    const dx = e.clientX - this.startX;
    const dy = e.clientY - this.startY;

    const minWidth = 350;
    const minHeight = 150;

    let newWidth = this.startWidth;
    let newHeight = this.startHeight;
    let newLeft = this.startLeft;
    let newTop = this.startTop;

    if (this.resizeDir.includes('e')) {
      newWidth = Math.max(minWidth, this.startWidth + dx);
    }
    if (this.resizeDir.includes('w')) {
      const possibleWidth = this.startWidth - dx;
      if (possibleWidth >= minWidth) {
        newWidth = possibleWidth;
        newLeft = this.startLeft + dx;
      } else {
        newWidth = minWidth;
        newLeft = this.startLeft + (this.startWidth - minWidth);
      }
    }

    if (this.resizeDir.includes('s')) {
      newHeight = Math.max(minHeight, this.startHeight + dy);
    }
    if (this.resizeDir.includes('n')) {
      const possibleHeight = this.startHeight - dy;
      if (possibleHeight >= minHeight) {
        newHeight = possibleHeight;
        newTop = this.startTop + dy;
      } else {
        newHeight = minHeight;
        newTop = this.startTop + (this.startHeight - minHeight);
      }
    }

    this.style.width = `${newWidth}px`;
    this.style.height = `${newHeight}px`;
    this.style.left = `${newLeft}px`;
    this.style.top = `${newTop}px`;
    this.style.right = 'auto';
    this.style.bottom = 'auto';
  };

  private onResizeMouseUp = (e: MouseEvent) => {
    if (this.isResizing) {
      e.preventDefault();
      this.isResizing = false;
      document.removeEventListener('mousemove', this.onResizeMouseMove);
      document.removeEventListener('mouseup', this.onResizeMouseUp);
    }
  };

  render() {
    return html`
      <div
        class="resize-handle n"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'n')}></div>
      <div
        class="resize-handle s"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 's')}></div>
      <div
        class="resize-handle e"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'e')}></div>
      <div
        class="resize-handle w"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'w')}></div>
      <div
        class="resize-handle ne"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'ne')}></div>
      <div
        class="resize-handle nw"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'nw')}></div>
      <div
        class="resize-handle se"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'se')}></div>
      <div
        class="resize-handle sw"
        @mousedown=${(e: MouseEvent) => this.onResizeMouseDown(e, 'sw')}></div>

      <div class="header" @mousedown=${this.onMouseDown}>
        <span>Triage Panel</span>
        <button
          class="collapse-btn"
          @click=${(e: Event) => {
            e.stopPropagation();
            this.collapsed = !this.collapsed;
            if (this.collapsed) {
              this.style.height = 'auto';
            }
          }}
          title=${this.collapsed ? 'Expand panel' : 'Collapse panel'}>
          ${this.collapsed ? '[+]' : '[-]'}
        </button>
      </div>

      <div class="panel-content" ?hidden=${this.collapsed}>
        <div class="add-bucket">
          <input
            type="text"
            placeholder="Add new bucket..."
            .value=${this.newBucketName}
            @input=${(e: Event) => (this.newBucketName = (e.target as HTMLInputElement).value)}
            @keydown=${this.onNewBucketKeyDown} />
          <button class="icon-btn" title="Add Bucket" @click=${this.addNewBucket}>
            <add-icon-sk></add-icon-sk>
          </button>
          <button
            class="icon-btn"
            title="Copy all panel buckets to clipboard"
            @click=${this.onCopyAll}>
            <content-copy-icon-sk></content-copy-icon-sk>
          </button>
          <button
            class="icon-btn"
            title="Paste panel state from clipboard"
            @click=${this.onPasteAll}>
            <content-paste-icon-sk></content-paste-icon-sk>
          </button>
        </div>

        <div class="buckets-grid">
          ${repeat(
            this.buckets,
            (b) => b.name,
            (bucket) => html`
              <triage-bucket-sk
                .bucket=${bucket}
                .anomalies=${this.anomalies}
                .traceNames=${this.traceNames}>
              </triage-bucket-sk>
            `
          )}
        </div>
      </div>
    `;
  }

  private onNewBucketKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      this.addNewBucket();
    }
  }

  addNewBucket() {
    if (this.bucketsController.addNewBucket(this.newBucketName)) {
      this.newBucketName = '';
    }
  }

  private onBucketUpdated(e: CustomEvent) {
    this.bucketsController.updateBucket(e.detail.bucket);
  }

  private onBucketDeleted(e: CustomEvent) {
    this.bucketsController.deleteBucket(e.detail.name);
  }

  addToBucket(name: string, anomaliesToAdd: Anomaly[]) {
    this.bucketsController.addToBucket(name, anomaliesToAdd);
  }

  getStagedAnomalyIds(): Set<string> {
    return this.bucketsController.getStagedAnomalyIds();
  }

  private onCopyAll() {
    this.bucketsController.copyAll();
  }

  private onPasteAll() {
    this.bucketsController.pasteAll(this.anomalies);
  }
}

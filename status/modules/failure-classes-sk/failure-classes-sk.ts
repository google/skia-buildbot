/**
 * @module modules/failure-classes-sk
 * @description <h2><code>failure-classes-sk</code></h2>
 *
 * Custom element to display active autogardener failure classes for a set of failed tasks.
 *
 * @event highlight-tasks - Dispatched when a failure class is hovered or a task belonging to a
 *   failure class is hovered. Detail has { taskIds: Array<string> }.
 * @event select-failure-class - Dispatched when a failure class item is clicked. Detail is the
 *   ActiveFailureClass.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface FailureClass {
  id: string;
  errorMessage: string;
  analysis: string;
  lastSeen?: string;
  repo?: string;
  classification?: string;
  culprits?: string[];
  resolved?: boolean;
}

export interface ActiveFailureClass {
  failureClass: FailureClass;
  taskIds: string[];
}

const FALLBACK_POLL_INTERVAL_MS = 30 * 1000;

export class FailureClassesSk extends ElementSk {
  private _taskIds: string[] = [];

  private classes: ActiveFailureClass[] = [];

  private taskToClass: Map<string, ActiveFailureClass> = new Map();

  private hoveredClassId: string = '';

  private refreshTimeout?: number;

  constructor() {
    super(FailureClassesSk.template);
  }

  private static template = (el: FailureClassesSk) => html`
    <div class="failure-classes-list">
      ${el.classes.length === 0
        ? html`<div class="empty">No active failure classes</div>`
        : el.classes.map(
            (item) => html`
              <div
                class="failure-class-item ${el.hoveredClassId === item.failureClass.id
                  ? 'hovered'
                  : ''}"
                data-failure-class-id=${item.failureClass.id}
                @mouseenter=${() => el.onClassMouseEnter(item)}
                @mouseleave=${() => el.onClassMouseLeave()}
                @click=${(e: Event) => {
                  e.stopPropagation();
                  el.selectClass(item);
                }}>
                <div class="analysis">${item.failureClass.analysis || '(No analysis)'}</div>
                <div class="count">
                  <span class="value" title="${item.taskIds.length} failed task(s)">
                    ${item.taskIds.length}
                  </span>
                </div>
              </div>
            `
          )}
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('taskIds');
    this._render();
    if (this._taskIds.length > 0) {
      this.refresh();
    }
  }

  disconnectedCallback(): void {
    this.clearRefreshTimeout();
  }

  get taskIds(): string[] {
    return this._taskIds;
  }

  set taskIds(ids: string[]) {
    const normalized = Array.from(new Set((ids || []).filter(Boolean))).sort();
    if (
      normalized.length === this._taskIds.length &&
      normalized.every((id, i) => id === this._taskIds[i])
    ) {
      return;
    }
    this._taskIds = normalized;
    this.clearRefreshTimeout();
    if (this._taskIds.length === 0) {
      this.classes = [];
      this.taskToClass.clear();
      this.hoveredClassId = '';
      this._render();
      return;
    }
    if (this.isConnected) {
      this.refresh();
    }
  }

  /**
   * Called when a task in the commits grid is hovered or unhovered.
   * Highlights the matching failure class in the sidebar and emits highlight-tasks
   * for all sibling tasks in that class.
   */
  setHoveredTask(taskId: string, hovered: boolean): void {
    const matched = hovered ? this.taskToClass.get(taskId) : undefined;
    const newHoveredId = matched ? matched.failureClass.id : '';
    if (this.hoveredClassId !== newHoveredId) {
      this.hoveredClassId = newHoveredId;
      this._render();
    }
    this.dispatchHighlightTasks(matched ? matched.taskIds : []);
  }

  private onClassMouseEnter(item: ActiveFailureClass): void {
    this.hoveredClassId = item.failureClass.id;
    this._render();
    this.dispatchHighlightTasks(item.taskIds);
  }

  private onClassMouseLeave(): void {
    this.hoveredClassId = '';
    this._render();
    this.dispatchHighlightTasks([]);
  }

  private dispatchHighlightTasks(taskIds: string[]): void {
    this.dispatchEvent(
      new CustomEvent('highlight-tasks', {
        bubbles: true,
        detail: { taskIds },
      })
    );
  }

  private selectClass(item: ActiveFailureClass): void {
    this.dispatchEvent(
      new CustomEvent<ActiveFailureClass>('select-failure-class', {
        bubbles: true,
        detail: item,
      })
    );
  }

  private clearRefreshTimeout(): void {
    if (this.refreshTimeout !== undefined) {
      window.clearTimeout(this.refreshTimeout);
      this.refreshTimeout = undefined;
    }
  }

  private refresh(): void {
    this.clearRefreshTimeout();
    if (this._taskIds.length === 0) {
      return;
    }
    const requestedIds = this._taskIds;
    fetch('/json/failure-classes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ taskIds: requestedIds }),
    })
      .then(jsonOrThrow)
      .then((json: ActiveFailureClass[]) => {
        // Ignore stale responses if taskIds changed while the request was in flight.
        if (this._taskIds !== requestedIds) {
          return;
        }
        this.classes = json || [];
        this.taskToClass.clear();
        let classifiedCount = 0;
        for (const item of this.classes) {
          for (const tid of item.taskIds || []) {
            this.taskToClass.set(tid, item);
            classifiedCount++;
          }
        }
        this._render();

        // If some failed tasks have not yet been classified by autogardener, poll again later.
        if (classifiedCount < this._taskIds.length && this.isConnected) {
          this.refreshTimeout = window.setTimeout(() => this.refresh(), FALLBACK_POLL_INTERVAL_MS);
        }
      })
      .catch(errorMessage);
  }
}

define('failure-classes-sk', FailureClassesSk);

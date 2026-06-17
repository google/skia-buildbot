import { LitElement, css, html, PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

export interface TourStep {
  selector: string;
  title: string;
  text: string;
  placement: 'top' | 'bottom' | 'left' | 'right';
}

@customElement('interactive-tour-sk')
export class InteractiveTourSk extends LitElement {
  @property({ type: Boolean }) active = false;

  @property({ type: Array }) steps: TourStep[] = [];

  @state() private _currentIndex = 0;

  @state() private _bubbleStyle = '';

  @state() private _spotlightStyle = '';

  static styles = css`
    .tour-overlay {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      z-index: 9999;
      pointer-events: none;
    }

    .tour-spotlight {
      position: fixed;
      box-shadow: 0 0 0 9999px rgb(15 23 42 / 55%);
      border-radius: 8px;
      border: 2px solid var(--primary, #6366f1);
      transition: all 0.3s ease;
      pointer-events: none;
    }

    .tour-bubble {
      position: fixed;
      width: 280px;
      background: #1e293b;
      border: 1px solid #334155;
      border-radius: 8px;
      padding: 16px;
      box-shadow: 0 20px 25px -5px rgb(0 0 0 / 30%);
      color: #f8fafc;
      z-index: 10000;
      transition: all 0.3s ease;
      display: flex;
      flex-direction: column;
      gap: 8px;
      pointer-events: auto;
    }

    .bubble-title {
      font-weight: 700;
      color: var(--primary, #818cf8);
      font-size: 14px;
      margin: 0;
    }

    .bubble-text {
      font-size: 12px;
      line-height: 1.5;
      margin: 0;
      color: #cbd5e1;
    }

    .tour-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-top: 12px;
      font-size: 11px;
    }

    .tour-nav-btns {
      display: flex;
      gap: 6px;
    }

    .tour-btn {
      background: none;
      border: none;
      color: #94a3b8;
      cursor: pointer;
      padding: 4px 8px;
      border-radius: 4px;
      font-weight: 600;
    }

    .tour-btn:hover {
      background: rgb(255 255 255 / 5%);
      color: #fff;
    }

    .tour-btn-primary {
      background: var(--primary, #6366f1);
      color: white;
    }

    .tour-btn-primary:hover {
      background: #4f46e5;
      color: white;
    }

    .tour-progress {
      color: #64748b;
      font-weight: 500;
    }
  `;

  private _scrollListener = () => this._updatePosition();

  protected updated(changedProperties: PropertyValues) {
    if (changedProperties.has('active')) {
      if (this.active) {
        window.addEventListener('scroll', this._scrollListener, { capture: true, passive: true });
      } else {
        window.removeEventListener('scroll', this._scrollListener, { capture: true });
      }
    }
    if (changedProperties.has('active') && this.active) {
      this._currentIndex = 0;
      this._updatePosition();
      this._dispatchStepChanged();
    }
    if (changedProperties.has('_currentIndex') && this.active) {
      this._updatePosition();
      this._dispatchStepChanged();
    }
  }

  private _dispatchStepChanged() {
    this.dispatchEvent(
      new CustomEvent('step-changed', {
        detail: { index: this._currentIndex },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _updatePosition(isRetry = false) {
    if (!this.active || this.steps.length === 0) return;
    const step = this.steps[this._currentIndex];

    // Probe document for targeted element
    const target = this._querySelectorDeep(step.selector);
    if (!target) {
      console.warn(`Tour target not found: ${step.selector}`);
      this._spotlightStyle = 'display: none;';
      this._bubbleStyle = 'top: 50%; left: 50%; transform: translate(-50%, -50%);';

      if (!isRetry) {
        setTimeout(() => this._updatePosition(true), 200);
      }
      return;
    }

    const rect = target.getBoundingClientRect();

    // Position spotlight cutout
    this._spotlightStyle = `
      top: ${rect.top - 4}px;
      left: ${rect.left - 4}px;
      width: ${rect.width + 8}px;
      height: ${rect.height + 8}px;
    `;

    // Position bubble popup
    let bubbleTop = 0;
    let bubbleLeft = 0;
    const margin = 12;
    const bubbleWidth = 280;

    if (step.placement === 'bottom') {
      bubbleTop = rect.bottom + margin;
      bubbleLeft = rect.left + rect.width / 2 - bubbleWidth / 2;
    } else if (step.placement === 'top') {
      bubbleTop = rect.top - 160 - margin; // approximate bubble height
      bubbleLeft = rect.left + rect.width / 2 - bubbleWidth / 2;
    }

    // Bound checks to keep on screen
    const bubbleHeight = 180; // approximate height
    bubbleTop = Math.max(10, Math.min(window.innerHeight - bubbleHeight - 10, bubbleTop));
    bubbleLeft = Math.max(10, Math.min(window.innerWidth - bubbleWidth - 10, bubbleLeft));

    this._bubbleStyle = `top: ${bubbleTop}px; left: ${bubbleLeft}px;`;
  }

  private _querySelectorDeep(
    selector: string,
    root: Document | ShadowRoot = document
  ): HTMLElement | null {
    const element = root.querySelector(selector) as HTMLElement;
    if (element) return element;

    const elements = root.querySelectorAll('*');
    for (let i = 0; i < elements.length; i++) {
      const el = elements[i];
      if (el.shadowRoot) {
        const found = this._querySelectorDeep(selector, el.shadowRoot);
        if (found) return found;
      }
    }

    return null;
  }

  private _onNext() {
    if (this._currentIndex < this.steps.length - 1) {
      this._currentIndex++;
    } else {
      this._onFinished();
    }
  }

  private _onPrev() {
    if (this._currentIndex > 0) {
      this._currentIndex--;
    }
  }

  private _onFinished() {
    this.active = false;
    this.dispatchEvent(new CustomEvent('tour-finished', { bubbles: true, composed: true }));
  }

  render() {
    if (!this.active || this.steps.length === 0) return html``;
    const step = this.steps[this._currentIndex];

    return html`
      <div class="tour-overlay">
        <div class="tour-spotlight" style=${this._spotlightStyle}></div>
        <div class="tour-bubble" style=${this._bubbleStyle}>
          <h4 class="bubble-title">${step.title}</h4>
          <p class="bubble-text">${step.text}</p>
          <div class="tour-footer">
            <button class="tour-btn tour-btn-skip" @click=${this._onFinished}>Skip</button>
            <span class="tour-progress">${this._currentIndex + 1}/${this.steps.length}</span>
            <div class="tour-nav-btns">
              ${this._currentIndex > 0
                ? html`<button class="tour-btn" @click=${this._onPrev}>Back</button>`
                : ''}
              <button class="tour-btn tour-btn-primary" @click=${this._onNext}>
                ${this._currentIndex === this.steps.length - 1 ? 'Finish' : 'Next'}
              </button>
            </div>
          </div>
        </div>
      </div>
    `;
  }
}

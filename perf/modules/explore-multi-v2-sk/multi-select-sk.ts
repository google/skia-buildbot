import { LitElement, html, css, PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';

export interface MultiSelectOption {
  value: string;
  count: number;
}

export interface MultiSelectSelectionEventDetail {
  value: string;
}

export interface MultiSelectSetSelectionEventDetail {
  values: string[];
}

export interface MultiSelectReplaceEventDetail {
  value: string;
}

@customElement('explore-multi-v2-select-sk')
export class MultiSelectSk extends LitElement {
  @property({ type: String }) label = '';

  @property({ type: Array }) options: MultiSelectOption[] = [];

  @property({ type: Array }) selected: string[] = [];

  @property({ type: String }) variant: 'default' | 'pill' = 'default';

  @property({ type: Boolean }) isSplit = false;

  @property({ type: Boolean }) showDiffButton = false;

  @property({ type: Boolean }) showSplitButton = false;

  @state() private _isOpen = false;

  @state() private _searchTerm = '';

  @state() private _focusedIndex = -1;

  @state() private _initialSelected: Set<string> = new Set();

  @state() private _hasClickedDuringGlob = false;

  @query('.multiselect-search input') private _inputElement?: HTMLInputElement;

  @query('.multiselect-list') private _listElement?: HTMLDivElement;

  static styles = css`
    :host {
      display: inline-block;
      font-family: var(--font, 'Inter', system-ui, sans-serif);
      position: relative;
    }

    .multiselect-container {
      position: relative;
    }

    .multiselect-container.default {
      display: flex;
      flex-direction: column;
      min-width: 200px;
    }

    .multiselect-container.pill {
      min-width: auto;
    }

    .multiselect-label {
      font-size: 11px;
      font-weight: 700;
      margin-bottom: 6px;
      color: var(--on-surface, #64748b);
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    .multiselect-trigger {
      border: 1px solid var(--outline, rgb(255 255 255 / 10%));
      border-radius: 8px;
      padding: 8px 12px;
      cursor: pointer;
      display: flex;
      justify-content: space-between;
      align-items: center;
      background: var(--background, #0f172a);
      color: var(--on-background, #f8fafc);
      user-select: none;
      transition: all 0.2s ease;
      min-height: 36px;
      box-sizing: border-box;
    }

    .multiselect-trigger:hover {
      background-color: var(--surface, rgb(255 255 255 / 5%));
      border-color: var(--outline, rgb(255 255 255 / 20%));
    }

    .multiselect-trigger.pill {
      border-radius: 16px;
      background: transparent;
      border: 1px solid var(--md-sys-color-outline, rgb(255 255 255 / 20%));
      padding: 4px 12px;
      gap: 6px;
      font-size: 13px;
      height: 28px;
      color: var(--on-surface, #cbd5e1);
    }

    .multiselect-trigger.pill:hover {
      background-color: rgb(128 128 128 / 20%);
      border-color: var(--md-sys-color-outline, rgb(255 255 255 / 30%));
    }

    .multiselect-trigger.open {
      border-color: var(--primary, #6366f1);
      box-shadow: 0 0 0 2px rgb(99 102 241 / 20%);
    }

    .pill-key {
      font-weight: 600;
      color: var(--on-surface, #94a3b8);
      white-space: nowrap;
    }

    .pill-remove {
      width: 16px;
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      margin-left: 2px;
      cursor: pointer;
      color: var(--on-surface, #94a3b8);
      font-size: 14px;
      transition: all 0.2s;
    }

    .pill-remove:hover {
      background-color: rgb(255 255 255 / 10%);
      color: var(--on-background, #fff);
    }

    .multiselect-value {
      flex: 1;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 500;
    }

    .multiselect-values-stack {
      display: flex;
      flex-direction: column;
      font-size: 11px;
      line-height: 1.2;
    }

    .more-values {
      font-style: italic;
      color: var(--on-surface, #64748b);
      margin-top: 2px;
    }

    .multiselect-arrow {
      font-size: 10px;
      margin-left: 8px;
      opacity: 0.5;
      transition: transform 0.2s;
    }

    .multiselect-trigger.open .multiselect-arrow {
      transform: rotate(180deg);
    }

    .multiselect-dropdown {
      position: absolute;
      top: calc(100% + 8px);
      left: 0;
      min-width: 300px;
      background: var(--surface, #1e293b);
      backdrop-filter: blur(12px);
      border-radius: 12px;
      box-shadow:
        0 10px 25px -5px rgb(0 0 0 / 50%),
        0 8px 10px -6px rgb(0 0 0 / 50%);
      z-index: 2000;
      overflow: hidden;
      display: flex;
      flex-direction: column;
      border: 1px solid var(--outline, rgb(255 255 255 / 5%));
    }

    .multiselect-search {
      padding: 12px;
      border-bottom: 1px solid var(--outline, rgb(255 255 255 / 5%));
      background-color: rgb(15 23 42 / 30%);
    }

    .multiselect-search input {
      width: 100%;
      padding: 8px 12px;
      border: 1px solid var(--outline, rgb(255 255 255 / 10%));
      border-radius: 6px;
      box-sizing: border-box;
      font-size: 13px;
      outline: none;
      background: var(--background, #0f172a);
      color: var(--on-background, #f8fafc);
      transition: border-color 0.2s;
    }

    .multiselect-search input:focus {
      border-color: var(--primary, #6366f1);
    }

    .multiselect-list {
      max-height: 300px;
      overflow-y: auto;
      padding: 6px 0;
    }

    .multiselect-option {
      padding: 8px 12px;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 12px;
      font-size: 13px;
      min-height: 36px;
      color: var(--on-surface, #cbd5e1);
      transition: all 0.1s ease;
    }

    .multiselect-option:hover,
    .multiselect-option.focused {
      background-color: rgb(255 255 255 / 3%);
      color: var(--on-background, #fff);
    }

    .multiselect-option.selected {
      background-color: rgb(99 102 241 / 10%);
      color: var(--primary, #818cf8);
    }

    .multiselect-option.preview {
      color: var(--on-surface, #64748b);
      background-color: rgb(255 255 255 / 1%);
      font-style: italic;
    }

    .ms-opt-checkbox {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
    }

    .ms-opt-text {
      flex: 1;
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      overflow: hidden;
    }

    .ms-opt-value {
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 500;
    }

    .ms-opt-count {
      color: var(--on-surface, #64748b);
      font-size: 11px;
      margin-left: 12px;
      flex-shrink: 0;
      font-family: monospace;
    }

    .ms-diff-btn {
      opacity: 0;
      padding: 4px 8px;
      font-size: 11px;
      border: 1px solid var(--outline, rgb(255 255 255 / 10%));
      border-radius: 4px;
      background: rgb(255 255 255 / 5%);
      color: var(--on-surface, #94a3b8);
      cursor: pointer;
      flex-shrink: 0;
      margin-left: 8px;
      transition: all 0.2s ease;
    }

    .multiselect-option:hover .ms-diff-btn,
    .multiselect-option.focused .ms-diff-btn {
      opacity: 1;
    }

    .ms-diff-btn:hover {
      background-color: rgb(99 102 241 / 20%);
      border-color: var(--primary, #6366f1);
      color: var(--on-background, #fff);
    }

    .multiselect-footer {
      padding: 12px;
      border-top: 1px solid var(--outline, rgb(255 255 255 / 5%));
      background-color: rgb(15 23 42 / 30%);
    }

    .ms-split-btn {
      width: 100%;
      padding: 8px;
      cursor: pointer;
      background-color: rgb(255 255 255 / 5%);
      border: 1px solid var(--outline, rgb(255 255 255 / 10%));
      border-radius: 6px;
      color: var(--on-surface, #cbd5e1);
      font-weight: 600;
      font-size: 12px;
      transition: all 0.2s ease;
    }

    .ms-split-btn:hover {
      background-color: rgb(255 255 255 / 10%);
      border-color: var(--outline, rgb(255 255 255 / 20%));
      color: var(--on-background, #fff);
    }

    .ms-split-btn.active {
      background-color: rgb(99 102 241 / 15%);
      border-color: rgb(99 102 241 / 30%);
      color: var(--primary, #818cf8);
    }

    .multiselect-option.empty {
      color: var(--on-surface, #64748b);
      font-style: italic;
      justify-content: center;
      cursor: default;
      padding: 24px;
    }

    /* Custom Checkbox */
    .custom-checkbox {
      position: relative;
      display: flex;
      align-items: center;
      cursor: pointer;
      user-select: none;
    }

    .custom-checkbox input {
      position: absolute;
      opacity: 0;
      cursor: pointer;
      height: 0;
      width: 0;
    }

    .checkmark {
      height: 16px;
      width: 16px;
      background-color: rgb(255 255 255 / 5%);
      border: 1px solid var(--outline, rgb(255 255 255 / 10%));
      border-radius: 4px;
      transition: all 0.2s;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .custom-checkbox:hover input ~ .checkmark {
      background-color: rgb(255 255 255 / 10%);
      border-color: var(--outline, rgb(255 255 255 / 20%));
    }

    .checkmark.checked {
      background-color: var(--primary, #6366f1);
      border-color: var(--primary, #6366f1);
    }

    .checkmark::after {
      content: '';
      display: none;
      width: 4px;
      height: 8px;
      border: solid white;
      border-width: 0 2px 2px 0;
      transform: rotate(45deg);
      margin-bottom: 2px;
    }

    .checkmark.checked::after {
      display: block;
    }
  `;

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('mousedown', this._handleClickOutside);
  }

  disconnectedCallback() {
    document.removeEventListener('mousedown', this._handleClickOutside);
    super.disconnectedCallback();
  }

  protected updated(changedProperties: PropertyValues) {
    if (changedProperties.has('_isOpen') && this._isOpen) {
      setTimeout(() => this._inputElement?.focus(), 0);
    }
  }

  private _handleClickOutside = (e: MouseEvent) => {
    if (this._isOpen && !e.composedPath().includes(this)) {
      this._isOpen = false;
    }
  };

  private _matchGlob(val: string, pattern: string): boolean {
    try {
      const parts = pattern
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      return parts.some((part) => {
        if (part.includes('*') || part.includes('?')) {
          const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
          const p = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
          const regex = new RegExp(p, 'i');
          return regex.test(val);
        }
        return val.toLowerCase() === part.toLowerCase();
      });
    } catch (_e) {
      return false;
    }
  }

  private _toggleOpen() {
    if (this.options.length <= 1 && this.variant !== 'pill') return;
    this._isOpen = !this._isOpen;

    if (this._isOpen) {
      this._focusedIndex = 0;
      this._searchTerm = '';
      this._initialSelected = new Set(this.selected);
      this._hasClickedDuringGlob = false;

      this.dispatchEvent(
        new CustomEvent('open', {
          bubbles: true,
          composed: true,
        })
      );
    }
  }

  private _getFilteredOptions() {
    let result = this.options;
    const isGlob =
      this._searchTerm.includes('*') ||
      this._searchTerm.includes('?') ||
      this._searchTerm.includes(',');

    if (this._searchTerm) {
      if (isGlob) {
        try {
          const parts = this._searchTerm
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean);
          const seen = new Set<string>();
          const results: MultiSelectOption[] = [];

          parts.forEach((part) => {
            let matches: MultiSelectOption[] = [];
            if (part.includes('*') || part.includes('?')) {
              const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
              const pattern = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
              const regex = new RegExp(pattern, 'i');
              matches = this.options.filter((opt) => regex.test(opt.value));
            } else {
              const lowerPart = part.toLowerCase();
              matches = this.options.filter((opt) => opt.value.toLowerCase().includes(lowerPart));
            }

            matches.forEach((opt) => {
              if (!seen.has(opt.value)) {
                seen.add(opt.value);
                results.push(opt);
              }
            });
          });
          result = results;
        } catch (e) {
          console.error(e);
        }
      } else {
        const lowerSearch = this._searchTerm.toLowerCase();
        result = this.options.filter((opt) => opt.value.toLowerCase().includes(lowerSearch));
      }
    }

    return [...result].sort((a, b) => {
      const aSel = this._initialSelected.has(a.value);
      const bSel = this._initialSelected.has(b.value);
      if (aSel && !bSel) return -1;
      if (!aSel && bSel) return 1;
      return 0;
    });
  }

  private _handleToggle(val: string) {
    const isGlob =
      this._searchTerm.includes('*') ||
      this._searchTerm.includes('?') ||
      this._searchTerm.includes(',');
    const filtered = this._getFilteredOptions();

    if (isGlob && !this._hasClickedDuringGlob) {
      this._hasClickedDuringGlob = true;
      const allMatches = filtered.map((o) => o.value);
      const isCurrentlySelectedInGlob = allMatches.includes(val);

      const newSelection = isCurrentlySelectedInGlob
        ? allMatches.filter((m) => m !== val)
        : [...allMatches, val];

      this.dispatchEvent(
        new CustomEvent<MultiSelectSetSelectionEventDetail>('set-selection', {
          detail: { values: newSelection },
          bubbles: true,
          composed: true,
        })
      );
      return;
    }

    const matchedGlob = this.selected.find(
      (pattern) =>
        (pattern.includes('*') || pattern.includes('?') || pattern.includes(',')) &&
        this._matchGlob(val, pattern)
    );

    if (matchedGlob) {
      const expandedMatches = this.options
        .map((o) => o.value)
        .filter((v) => this._matchGlob(v, matchedGlob) && v !== val);

      const newSelection = [...this.selected.filter((v) => v !== matchedGlob), ...expandedMatches];

      this.dispatchEvent(
        new CustomEvent<MultiSelectSetSelectionEventDetail>('set-selection', {
          detail: { values: newSelection },
          bubbles: true,
          composed: true,
        })
      );
      return;
    }

    this.dispatchEvent(
      new CustomEvent<MultiSelectSelectionEventDetail>('selection-change', {
        detail: { value: val },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleKeyDown(e: KeyboardEvent) {
    if (!this._isOpen) {
      if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
        e.preventDefault();
        this._isOpen = true;
      }
      return;
    }

    const filtered = this._getFilteredOptions();

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        this._focusedIndex = (this._focusedIndex + 1) % (filtered.length || 1);
        this._scrollFocusedIntoView();
        break;
      case 'ArrowUp':
        e.preventDefault();
        this._focusedIndex =
          (this._focusedIndex - 1 + (filtered.length || 1)) % (filtered.length || 1);
        this._scrollFocusedIntoView();
        break;
      case ' ':
        e.preventDefault();
        if (this._focusedIndex >= 0 && this._focusedIndex < filtered.length) {
          this._handleToggle(filtered[this._focusedIndex].value);
        }
        break;
      case 'Enter':
        e.preventDefault();
        const isGlob =
          this._searchTerm.includes('*') ||
          this._searchTerm.includes('?') ||
          this._searchTerm.includes(',');
        if (isGlob && !this._hasClickedDuringGlob && this._searchTerm.trim()) {
          this.dispatchEvent(
            new CustomEvent<MultiSelectReplaceEventDetail>('replace-selection', {
              detail: { value: this._searchTerm },
              bubbles: true,
              composed: true,
            })
          );
          this._isOpen = false;
          return;
        }

        if (this._focusedIndex >= 0 && this._focusedIndex < filtered.length) {
          this._handleToggle(filtered[this._focusedIndex].value);
        }
        this._isOpen = false;
        break;
      case 'Escape':
        e.preventDefault();
        this._isOpen = false;
        break;
      case 'Tab':
        this._isOpen = false;
        break;
    }
  }

  private _scrollFocusedIntoView() {
    setTimeout(() => {
      if (this._listElement && this._focusedIndex >= 0) {
        const item = this._listElement.children[this._focusedIndex] as HTMLElement;
        if (item) {
          item.scrollIntoView({ block: 'nearest' });
        }
      }
    }, 0);
  }

  private _renderLabel() {
    if (this.selected.length === 0) return html`<span class="placeholder">(Any)</span>`;

    if (this.variant === 'pill') {
      if (this.selected.length === 1) return this.selected[0];

      const displayed = this.selected.length > 4 ? this.selected.slice(0, 3) : this.selected;
      const remaining = this.selected.length - displayed.length;

      return html`
        <div class="multiselect-values-stack">
          ${displayed.map((val) => html`<div>${val}</div>`)}
          ${remaining > 0 ? html`<div class="more-values">and ${remaining} more values</div>` : ''}
        </div>
      `;
    }

    if (this.selected.length === 1) return this.selected[0];
    return `${this.selected.length} selected`;
  }

  private _renderPillKey() {
    if (this.variant !== 'pill') return '';
    return html`<div class="pill-key">${this.label + '='}</div>`;
  }

  render() {
    const filtered = this._getFilteredOptions();
    const isGlob =
      this._searchTerm.includes('*') ||
      this._searchTerm.includes('?') ||
      this._searchTerm.includes(',');

    return html`
      <div
        class="${this.variant === 'pill'
          ? 'multiselect-container pill'
          : 'multiselect-container default'}">
        ${this.variant === 'default'
          ? html`<label class="multiselect-label">${this.label}</label>`
          : ''}

        <div
          class="${this._isOpen
            ? `multiselect-trigger ${this.variant} open`
            : `multiselect-trigger ${this.variant}`}"
          @click=${this._toggleOpen}
          tabindex="0"
          @keydown=${this._handleKeyDown}>
          ${this._renderPillKey()}
          <div class="multiselect-value">${this._renderLabel()}</div>

          ${this.variant === 'default' ? html`<div class="multiselect-arrow">▼</div>` : ''}
          ${this.variant === 'pill'
            ? html`
                <span
                  class="pill-remove"
                  @click=${(e: Event) => {
                    e.stopPropagation();
                    this.dispatchEvent(
                      new CustomEvent('remove-key', { bubbles: true, composed: true })
                    );
                  }}
                  >×</span
                >
              `
            : ''}
        </div>

        ${this._isOpen
          ? html`
              <div class="multiselect-dropdown">
                <div class="multiselect-search">
                  <input
                    type="text"
                    placeholder="Search..."
                    .value=${this._searchTerm}
                    @input=${(e: InputEvent) => {
                      this._searchTerm = (e.target as HTMLInputElement).value;
                      this._focusedIndex = 0;
                    }}
                    @click=${(e: Event) => e.stopPropagation()}
                    @keydown=${this._handleKeyDown} />
                </div>
                <div class="multiselect-list">
                  ${filtered.length === 0
                    ? html`
                        <div class="multiselect-option empty">
                          No options match "${this._searchTerm}"
                        </div>
                      `
                    : filtered.map((option, index) => {
                        const isSelected =
                          this.selected.includes(option.value) ||
                          this.selected.some(
                            (p) =>
                              (p.includes('*') || p.includes('?') || p.includes(',')) &&
                              this._matchGlob(option.value, p)
                          );
                        const isFocused = index === this._focusedIndex;
                        const isPreviewMatch =
                          isGlob && !this._hasClickedDuringGlob && !!this._searchTerm;
                        const isEffectivelySelected = isPreviewMatch || isSelected;

                        return html`
                          <div
                            class="multiselect-option ${isSelected && !isPreviewMatch
                              ? 'selected'
                              : ''} ${isFocused ? 'focused' : ''} ${isPreviewMatch
                              ? 'preview'
                              : ''}"
                            @click=${(e: Event) => {
                              e.stopPropagation();
                              this._handleToggle(option.value);
                            }}
                            @mouseenter=${() => {
                              if (this._focusedIndex !== index) {
                                this._focusedIndex = index;
                              }
                            }}>
                            <div class="ms-opt-checkbox">
                              <div class="custom-checkbox">
                                <span
                                  class="checkmark ${isEffectivelySelected
                                    ? 'checked'
                                    : ''}"></span>
                              </div>
                            </div>
                            <span class="ms-opt-text">
                              <span class="ms-opt-value">${option.value}</span>
                              <span class="ms-opt-count">${option.count.toLocaleString()}</span>
                            </span>
                            ${this.showDiffButton
                              ? html`
                                  <button
                                    class="ms-diff-btn"
                                    @click=${(e: Event) => {
                                      e.stopPropagation();
                                      this.dispatchEvent(
                                        new CustomEvent('diff-base', {
                                          detail: { key: this.label, value: option.value },
                                          bubbles: true,
                                          composed: true,
                                        })
                                      );
                                      this._isOpen = false;
                                    }}
                                    title="Set as Diff Base">
                                    Diff
                                  </button>
                                `
                              : ''}
                          </div>
                        `;
                      })}
                </div>
                ${this.showSplitButton
                  ? html`
                      <div class="multiselect-footer">
                        <button
                          class="ms-split-btn ${this.isSplit ? 'active' : ''}"
                          @click=${(e: Event) => {
                            e.stopPropagation();
                            this.dispatchEvent(
                              new CustomEvent('split', { bubbles: true, composed: true })
                            );
                            this._isOpen = false;
                          }}>
                          ${this.isSplit ? `Stop Splitting` : `Split by ${this.label}`}
                        </button>
                      </div>
                    `
                  : ''}
              </div>
            `
          : ''}
      </div>
    `;
  }
}

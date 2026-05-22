import { LitElement, css, html } from 'lit';
import { customElement, state } from 'lit/decorators.js';

interface ShortcutItem {
  keys: string[];
  desc: string;
}

interface FaqItem {
  q: string;
  a: string;
}

@customElement('help-hub-sk')
export class HelpHubSk extends LitElement {
  @state() private _open = false;

  @state() private _activeTab: 'guides' | 'shortcuts' | 'faqs' = 'guides';

  @state() private _searchQuery = '';

  private _shortcuts: ShortcutItem[] = [
    { keys: ['Cmd', 'Z'], desc: 'Reset zoom viewport to original boundaries' },
    { keys: ['Shift', 'Drag'], desc: 'Zoom in on selected x-axis range' },
    { keys: ['Click Point'], desc: 'Pin commit point and display details' },
    { keys: ['Double Click'], desc: 'Reset chart zoom boundaries' },
  ];

  private _faqs: FaqItem[] = [
    {
      q: 'How do I perform split chart analyses?',
      a: 'Enter query parameters, select a split key (e.g., bot name), and enable "Split" chart. It creates individual graphs per bot.',
    },
    {
      q: 'How to compare metrics with a baseline?',
      a: 'Find the baseline trace inside the query bar options and trigger "Diff Base". All other lines will show difference levels.',
    },
    {
      q: 'Why is my page loading slowly?',
      a: 'High-density queries overlaying thousands of trace elements are throttled. Consider splitting chart or filtering subrepos.',
    },
  ];

  static styles = css`
    .help-fab {
      position: fixed;
      bottom: 24px;
      right: 24px;
      width: 50px;
      height: 50px;
      border-radius: 50%;
      background: var(--primary, #6366f1);
      color: white;
      border: none;
      font-size: 20px;
      font-weight: bold;
      cursor: pointer;
      box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);
      z-index: 999;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: all 0.2s;
    }

    .help-fab:hover {
      transform: scale(1.05);
      box-shadow: 0 6px 20px rgba(99, 102, 241, 0.6);
    }

    .help-panel {
      position: fixed;
      top: 0;
      right: -390px;
      width: 360px;
      height: 100vh;
      background: rgba(15, 23, 42, 0.9);
      backdrop-filter: blur(16px);
      border-left: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow: -10px 0 30px rgba(0, 0, 0, 0.3);
      z-index: 1000;
      transition: right 0.3s ease-in-out;
      display: flex;
      flex-direction: column;
    }

    .help-panel.open {
      right: 0;
    }

    .panel-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px;
      border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    }

    .panel-title {
      margin: 0;
      font-size: 16px;
      font-weight: 700;
      color: var(--primary, #818cf8);
    }

    .close-btn {
      background: none;
      border: none;
      color: #94a3b8;
      font-size: 24px;
      cursor: pointer;
    }

    .close-btn:hover {
      color: #fff;
    }

    .panel-content {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 16px;
    }

    .search-bar {
      width: 95%;
      padding: 8px 10px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 6px;
      color: #fff;
      font-size: 12px;
    }

    .tabs-nav {
      display: flex;
      gap: 12px;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
      padding-bottom: 8px;
      font-size: 12px;
    }

    .tab-item {
      color: #94a3b8;
      cursor: pointer;
      font-weight: 600;
    }

    .tab-item.active {
      color: var(--primary, #818cf8);
      border-bottom: 2px solid var(--primary, #818cf8);
      padding-bottom: 6px;
    }

    .tour-card {
      background: rgba(99, 102, 241, 0.15);
      border: 1px solid rgba(99, 102, 241, 0.3);
      border-radius: 8px;
      padding: 14px;
      text-align: center;
    }

    .tour-card h4 {
      margin: 0 0 6px 0;
      color: #a5b4fc;
      font-size: 13px;
    }

    .tour-card p {
      margin: 0 0 12px 0;
      font-size: 11px;
      color: #cbd5e1;
      line-height: 1.4;
    }

    .tour-trigger-btn {
      background: var(--primary, #6366f1);
      color: white;
      border: none;
      padding: 6px 14px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: bold;
      cursor: pointer;
    }

    .tour-trigger-btn:hover {
      background: #4f46e5;
    }

    .shortcut-list,
    .faq-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .shortcut-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 8px;
      background: rgba(255, 255, 255, 0.02);
      border-radius: 6px;
      font-size: 12px;
    }

    .keys-badge kbd {
      background: #334155;
      border: 1px solid #475569;
      border-radius: 3px;
      padding: 2px 4px;
      font-size: 10px;
      font-family: monospace;
      color: #f8fafc;
      margin-left: 4px;
    }

    .faq-item {
      background: rgba(255, 255, 255, 0.02);
      padding: 10px;
      border-radius: 6px;
    }

    .faq-q {
      font-weight: bold;
      font-size: 12px;
      color: #cbd5e1;
      margin-bottom: 4px;
    }

    .faq-a {
      font-size: 11px;
      color: #94a3b8;
      line-height: 1.4;
    }

    .guides-section {
      display: flex;
      flex-direction: column;
      gap: 12px;
      margin-top: 8px;
    }

    .section-subtitle {
      margin: 0;
      font-size: 12px;
      font-weight: 700;
      color: #a5b4fc;
    }

    .recipe-card {
      background: rgba(255, 255, 255, 0.02);
      border: 1px solid rgba(255, 255, 255, 0.05);
      border-radius: 6px;
      padding: 10px;
    }

    .recipe-header {
      font-weight: 600;
      font-size: 11px;
      color: #f1f5f9;
      margin-bottom: 4px;
    }

    .recipe-body p {
      margin: 0;
      font-size: 11px;
      color: #94a3b8;
      line-height: 1.4;
    }

    .recipe-body code {
      background: rgba(255, 255, 255, 0.08);
      padding: 2px 4px;
      border-radius: 3px;
      font-family: monospace;
      color: #f472b6;
    }
  `;

  openPanel() {
    this._open = true;
  }

  private _togglePanel() {
    this._open = !this._open;
  }

  private _onStartTour() {
    this._open = false;
    this.dispatchEvent(new CustomEvent('start-tour', { bubbles: true, composed: true }));
  }

  private _onApplyRandomPreset() {
    this._open = false;
    this.dispatchEvent(new CustomEvent('request-random-preset', { bubbles: true, composed: true }));
  }

  private _onApplyComparisonPreset() {
    this._open = false;
    this.dispatchEvent(
      new CustomEvent('request-comparison-preset', { bubbles: true, composed: true })
    );
  }

  private _onSearchInput(e: any) {
    this._searchQuery = e.target.value;
  }

  private _onSelectGuidesTab() {
    this._activeTab = 'guides';
  }

  private _onSelectShortcutsTab() {
    this._activeTab = 'shortcuts';
  }

  private _onSelectFaqsTab() {
    this._activeTab = 'faqs';
  }

  render() {
    const query = this._searchQuery.toLowerCase();
    const filteredShortcuts = this._shortcuts.filter(
      (s) =>
        s.desc.toLowerCase().includes(query) || s.keys.some((k) => k.toLowerCase().includes(query))
    );
    const filteredFaqs = this._faqs.filter(
      (f) => f.q.toLowerCase().includes(query) || f.a.toLowerCase().includes(query)
    );

    return html`
      <button class="help-fab" @click=${this._togglePanel} title="Help Hub">?</button>

      <div class="help-panel ${this._open ? 'open' : ''}">
        <div class="panel-header">
          <h3 class="panel-title">Explore Multi Help Hub</h3>
          <button class="close-btn" @click=${this._togglePanel}>&times;</button>
        </div>

        <div class="panel-content">
          <input
            type="text"
            class="search-bar"
            placeholder="Search help &amp; shortcuts..."
            .value=${this._searchQuery}
            @input=${this._onSearchInput} />

          <div class="tabs-nav">
            <span
              class="tab-item ${this._activeTab === 'guides' ? 'active' : ''}"
              @click=${this._onSelectGuidesTab}
              >Guides</span
            >
            <span
              class="tab-item ${this._activeTab === 'shortcuts' ? 'active' : ''}"
              @click=${this._onSelectShortcutsTab}
              >Shortcuts</span
            >
            <span
              class="tab-item ${this._activeTab === 'faqs' ? 'active' : ''}"
              @click=${this._onSelectFaqsTab}
              >FAQs</span
            >
          </div>

          ${this._activeTab === 'guides'
            ? html`
                <div class="tour-card">
                  <h4>New to Explore Multi V2?</h4>
                  <p>
                    Let us take you on a quick 4-step tour of the dimensions analysis dashboard to
                    get you acquainted.
                  </p>
                  <button class="tour-trigger-btn" @click=${this._onStartTour}>
                    Start Interface Tour
                  </button>
                </div>
                <div class="guides-section">
                  <h5 class="section-subtitle">💡 Quick Search & Typing Recipes</h5>

                  <div class="recipe-card">
                    <div class="recipe-header">
                      <span class="recipe-title">1. Basic Dimension Filtering</span>
                    </div>
                    <div class="recipe-body">
                      <p>
                        Type dimension keys and values directly. E.g. typing
                        <code>master=ChromiumPerf</code> and pressing <strong>Enter</strong> filters
                        for that performance test suite. Autocomplete suggestions update instantly
                        as you type.
                      </p>
                    </div>
                  </div>

                  <div class="recipe-card">
                    <div class="recipe-header">
                      <span class="recipe-title">2. Multi-Dimension Search</span>
                    </div>
                    <div class="recipe-body">
                      <p>
                        To narrow down system runs, separate multiple dimensions with spaces. E.g.
                        type <code>master=ChromiumAndroid bot=win-10_amd_laptop-perf</code> to find
                        Windows-based Android performance traces in a single bar.
                      </p>
                    </div>
                  </div>

                  <div class="recipe-card">
                    <div class="recipe-header">
                      <span class="recipe-title">3. Compare Multiple Query Rows</span>
                    </div>
                    <div class="recipe-body">
                      <p>
                        Click the <strong>➕ Add Query Row</strong> button to compare different
                        datasets. By default, all active query lines are overlaid together on a
                        <strong>single graph</strong> for comparison. To stack them into separate
                        individual graphs instead, simply turn on the
                        <strong>Split Chart</strong> toggle in the toolbar!
                      </p>
                    </div>
                  </div>

                  <div class="recipe-card">
                    <div class="recipe-header">
                      <span class="recipe-title">4. Advanced Autocomplete & Glob Previews</span>
                    </div>
                    <div class="recipe-body">
                      <p>
                        The query suggestion engine is extremely smart! Type
                        <code>bot=*laptop*</code> in the search bar to perform wildcard/glob
                        matching across all laptop performance bots. Furthermore, clicking on any
                        query pill opens the custom <strong>Multi-Select Dropdown</strong>; typing a
                        glob like <code>*memory*</code> in its search bar displays a live
                        <i>italicized preview</i> of all matching memory benchmarks, allowing you to
                        press <strong>Enter</strong> to select them collectively!
                      </p>
                    </div>
                  </div>

                  <h5 class="section-subtitle" style="margin-top: 8px;">
                    🚀 Try It Out: Live Query Presets
                  </h5>

                  <div
                    class="recipe-card"
                    style="background: rgba(99, 102, 241, 0.08); border-color: rgba(99, 102, 241, 0.2);">
                    <div
                      class="recipe-header"
                      style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="recipe-title" style="color: #a5b4fc;"
                        >🎯 Load Random Query Preset</span
                      >
                      <button
                        class="tour-trigger-btn"
                        style="padding: 2px 8px; font-size: 9px;"
                        @click=${this._onApplyRandomPreset}>
                        Load ▶️
                      </button>
                    </div>
                    <div class="recipe-body">
                      <p style="font-size: 10px;">
                        Pulls a real, guaranteed-to-exist trace query dynamically from the
                        background Web Worker.
                      </p>
                    </div>
                  </div>

                  <div
                    class="recipe-card"
                    style="background: rgba(99, 102, 241, 0.08); border-color: rgba(99, 102, 241, 0.2);">
                    <div
                      class="recipe-header"
                      style="display: flex; justify-content: space-between; align-items: center;">
                      <span class="recipe-title" style="color: #a5b4fc;"
                        >📊 Load Multi-Row Comparison</span
                      >
                      <button
                        class="tour-trigger-btn"
                        style="padding: 2px 8px; font-size: 9px;"
                        @click=${this._onApplyComparisonPreset}>
                        Load ▶️
                      </button>
                    </div>
                    <div class="recipe-body">
                      <p style="font-size: 10px;">
                        Pulls a real trace from the worker, selects one of its dimension keys, and
                        overlays two comparative rows.
                      </p>
                    </div>
                  </div>
                </div>
              `
            : ''}
          ${this._activeTab === 'shortcuts'
            ? html`
                <div class="shortcut-list">
                  ${filteredShortcuts.map(
                    (s) => html`
                      <div class="shortcut-row">
                        <span>${s.desc}</span>
                        <span class="keys-badge">
                          ${s.keys.map((k) => html`<kbd>${k}</kbd>`)}
                        </span>
                      </div>
                    `
                  )}
                </div>
              `
            : ''}
          ${this._activeTab === 'faqs'
            ? html`
                <div class="faq-list">
                  ${filteredFaqs.map(
                    (f) => html`
                      <div class="faq-item">
                        <div class="faq-q">${f.q}</div>
                        <div class="faq-a">${f.a}</div>
                      </div>
                    `
                  )}
                </div>
              `
            : ''}
        </div>
      </div>
    `;
  }
}

// perfinsights pages: the landing page (<body data-page="home">) and the trace viewer
// (<body data-page="trace">). Builds the DOM with createElement and textContent only, never from
// HTML strings, so API data can't inject markup.
'use strict';

(function () {
  // ---------------------------------------------------------------------------------------------
  // Shared helpers.

  /**
   * The path the site is mounted under, e.g. /insights, from <body data-base>. Only plain path
   * segments are accepted, so requests and links always stay on this origin.
   */
  let BASE = '';

  function readBase() {
    const base = (document.body && document.body.dataset && document.body.dataset.base) || '';
    return /^(\/[A-Za-z0-9_-]+)*$/.test(base) ? base : '';
  }

  /** Returns a new element with optional text and class. */
  function el(tag, text, className) {
    const e = document.createElement(tag);
    if (text !== undefined && text !== null) {
      e.textContent = String(text);
    }
    if (className) {
      e.className = className;
    }
    return e;
  }

  /**
   * Returns a link that opens in a new tab, or a plain span when href is not an http(s) URL.
   * className is only set on links.
   */
  function link(href, text, className) {
    if (!href) {
      return el('span', text);
    }
    let url = null;
    try {
      url = new URL(href, window.location.origin);
    } catch (e) {
      url = null;
    }
    if (!url || (url.protocol !== 'https:' && url.protocol !== 'http:')) {
      return el('span', text);
    }
    const a = el('a', text, className);
    a.href = url.href;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    return a;
  }

  /** Returns the URL of the trace viewer page of an anomaly. */
  function traceURL(id) {
    return BASE + '/trace?' + new URLSearchParams({ id: String(id) }).toString();
  }

  /** Returns a link that opens the trace viewer of an anomaly in a new tab. */
  function traceLink(id) {
    const a = el('a', 'Trace', 'button-link small');
    a.href = traceURL(id);
    a.target = '_blank';
    a.rel = 'noopener';
    a.title = 'Open the trace and details of this anomaly in a new tab';
    return a;
  }

  /** Fetches JSON from this server and turns error envelopes into exceptions. */
  async function getJSON(path) {
    const resp = await fetch(BASE + path, {
      headers: { Accept: 'application/json' },
      credentials: 'same-origin',
    });
    let body = null;
    try {
      body = await resp.json();
    } catch (e) {
      body = null;
    }
    if (!resp.ok) {
      const err = body && body.error;
      let msg = err ? err.code + ': ' + err.message : 'HTTP ' + resp.status;
      if (err && err.hint) {
        msg += ' (' + err.hint + ')';
      }
      throw new Error(msg);
    }
    return body;
  }

  function setStatusIn(id, text, isError) {
    const status = document.getElementById(id);
    status.textContent = text || '';
    status.classList.toggle('error', !!isError);
  }

  function formatPct(p) {
    if (p === null || p === undefined) {
      return 'n/a';
    }
    return (p > 0 ? '+' : '') + p.toFixed(1) + '%';
  }

  /** Formats a measurement compactly for axis labels, tooltips and tables. */
  function formatValue(v) {
    if (!Number.isFinite(v)) {
      return 'n/a';
    }
    const a = Math.abs(v);
    if (a !== 0 && (a >= 1e6 || a < 1e-3)) {
      return v.toExponential(2);
    }
    return String(Number(v.toPrecision(4)));
  }

  /** Formats a time (an RFC 3339 string or Unix seconds) in the browser's time zone. */
  function formatTime(t) {
    if (t === undefined || t === null || t === '') {
      return '';
    }
    const d = typeof t === 'number' ? new Date(t * 1000) : new Date(String(t));
    if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) {
      return ''; // Unknown, e.g. Go's zero time.
    }
    return d.toLocaleString();
  }

  /** Copies text to the clipboard and reports it on the button. */
  async function copyText(button, text) {
    const label = button.textContent;
    try {
      await navigator.clipboard.writeText(text);
      button.textContent = 'Copied';
    } catch (e) {
      button.textContent = 'Copy failed';
    }
    setTimeout(() => {
      button.textContent = label;
    }, 1500);
  }

  // ---------------------------------------------------------------------------------------------
  // Landing page.

  function setStatus(text, isError) {
    setStatusIn('try-status', text, isError);
  }

  function stat(label, value) {
    const d = el('div', null, 'stat');
    d.appendChild(el('div', value, 'stat-value'));
    d.appendChild(el('div', label, 'stat-label'));
    return d;
  }

  function renderSummary(summary) {
    const out = document.getElementById('try-result');
    out.replaceChildren();
    const t = summary.totals || {};

    const stats = el('div', null, 'stats');
    stats.appendChild(stat('regressions', t.regressions || 0));
    stats.appendChild(stat('improvements', t.improvements || 0));
    stats.appendChild(stat('untriaged', t.untriaged || 0));
    stats.appendChild(stat('triaged', t.triaged || 0));
    stats.appendChild(stat('ignored', t.ignored || 0));
    out.appendChild(stats);

    const scope = summary.scope || {};
    let scopeText =
      'Scanned ' + (scope.total_scanned || 0) + ' anomalies since ' + scope.since + '.';
    if (scope.truncated) {
      scopeText += ' The scan limit was reached, so counts are partial.';
    }
    out.appendChild(el('p', scopeText, 'muted'));

    const top = summary.top_regressions || [];
    out.appendChild(el('h3', 'Biggest regressions'));
    if (top.length === 0) {
      out.appendChild(el('p', 'No regressions in this period.', 'muted'));
    } else {
      const list = el('ul', null, 'anomalies');
      for (const a of top) {
        const li = el('li');
        li.appendChild(el('span', formatPct(a.pct_change), 'pct'));
        li.appendChild(el('span', a.headline, 'headline'));
        if (a.links && a.links.report) {
          li.appendChild(document.createTextNode(' '));
          li.appendChild(link(a.links.report, 'report'));
        }
        li.appendChild(traceLink(a.id));
        li.appendChild(el('div', 'id ' + a.id, 'id'));
        list.appendChild(li);
      }
      out.appendChild(list);
    }

    const overlaps = summary.overlapping_ranges || [];
    if (overlaps.length > 0) {
      out.appendChild(el('h3', 'Overlapping commit ranges (may share a root cause)'));
      const list = el('ul', null, 'overlaps');
      for (const o of overlaps) {
        const range = o.commit_range || [];
        list.appendChild(
          el(
            'li',
            'Commits ' +
              range[0] +
              '\u2013' +
              range[1] +
              ': ' +
              o.count +
              ' regressions in ' +
              (o.benchmarks || []).join(', ')
          )
        );
      }
      out.appendChild(list);
    }

    const raw = el('details', null, 'raw');
    raw.appendChild(el('summary', 'Raw JSON (what an agent receives)'));
    raw.appendChild(el('pre', JSON.stringify(summary, null, 2)));
    out.appendChild(raw);
  }

  /** Labels of the triage suggestion actions; unknown actions are shown as "review". */
  const ACTION_LABELS = {
    file_one_bug: 'File one bug',
    file_bug: 'File a bug',
    ignore_likely_noise: 'Likely noise',
    review_improvements: 'Improvements',
    review: 'Review',
  };

  function renderSuggestion(sug) {
    const action = Object.prototype.hasOwnProperty.call(ACTION_LABELS, sug.action)
      ? sug.action
      : 'review';
    const card = el('div', null, 'suggestion action-' + action.replace(/_/g, '-'));
    const head = el('div', null, 'suggestion-head');
    head.appendChild(el('span', ACTION_LABELS[action], 'badge-action'));
    head.appendChild(el('strong', sug.title));
    card.appendChild(head);
    card.appendChild(el('p', sug.reason, 'muted'));

    const meta = [];
    const cr = sug.commit_range;
    if (Array.isArray(cr) && cr.length === 2) {
      meta.push(cr[0] === cr[1] ? 'commit ' + cr[0] : 'commits ' + cr[0] + '\u2013' + cr[1]);
    }
    if (sug.benchmarks && sug.benchmarks.length) {
      meta.push('benchmarks: ' + sug.benchmarks.join(', '));
    }
    if (sug.bots && sug.bots.length) {
      meta.push('bots: ' + sug.bots.join(', '));
    }
    if (meta.length) {
      card.appendChild(el('div', meta.join(' \u00b7 '), 'suggestion-meta'));
    }
    const rep = sug.representative;
    if (rep) {
      card.appendChild(
        el('div', (sug.count > 1 ? 'Biggest: ' : '') + rep.headline, 'suggestion-rep')
      );
    }

    const actions = el('div', null, 'suggestion-actions');
    const ids = sug.anomaly_ids || [];
    if (sug.triage_url) {
      const partial = sug.count > ids.length && !/[?&]rev=/.test(sug.triage_url);
      actions.appendChild(
        link(
          sug.triage_url,
          partial ? 'Open the first ' + ids.length + ' in Perf' : 'Open in Perf to triage'
        )
      );
    }
    if (rep && rep.id) {
      actions.appendChild(traceLink(rep.id));
    }
    const draft = sug.bug_draft;
    if (draft) {
      const draftText = draft.title + '\n\n' + draft.description;
      const copyButton = el('button', 'Copy bug draft', 'small');
      copyButton.type = 'button';
      copyButton.addEventListener('click', () => copyText(copyButton, draftText));
      actions.appendChild(copyButton);
    }
    card.appendChild(actions);
    if (draft) {
      const details = el('details', null, 'bug-draft');
      details.appendChild(el('summary', 'Bug draft'));
      details.appendChild(el('pre', draft.title + '\n\n' + draft.description));
      card.appendChild(details);
    }
    return card;
  }

  function renderTriage(plan) {
    const out = document.getElementById('triage-result');
    out.replaceChildren();
    const scope = plan.scope || {};
    let text =
      plan.untriaged +
      ' untriaged anomalies in the last ' +
      scope.days +
      (scope.days === 1 ? ' day' : ' days') +
      ' (scanned ' +
      scope.total_scanned +
      ').';
    if (scope.truncated) {
      text += ' The scan limit was reached: pick fewer days to see them all.';
    }
    out.appendChild(el('p', text));
    const suggestions = plan.suggestions || [];
    if (suggestions.length === 0) {
      out.appendChild(el('p', 'Nothing to triage in this period.', 'muted'));
    }
    for (const sug of suggestions) {
      out.appendChild(renderSuggestion(sug));
    }
    if (plan.note) {
      out.appendChild(el('p', plan.note, 'muted note'));
    }
  }

  async function runTriage() {
    const params = new URLSearchParams({
      sheriff: document.getElementById('try-sheriff').value,
      days: document.getElementById('triage-days').value || '1',
    });
    setStatusIn('triage-status', 'Loading\u2026');
    try {
      const plan = await getJSON('/api/v1/insights:suggestTriage?' + params.toString());
      setStatusIn('triage-status', '');
      renderTriage(plan);
    } catch (e) {
      document.getElementById('triage-result').replaceChildren();
      setStatusIn('triage-status', e.message, true);
    }
  }

  async function runSummary() {
    const sheriff = document.getElementById('try-sheriff').value;
    const days = document.getElementById('try-days').value || '7';
    const params = new URLSearchParams({ sheriff: sheriff, days: days });
    setStatus('Loading\u2026');
    try {
      const summary = await getJSON('/api/v1/insights:summarize?' + params.toString());
      setStatus('');
      renderSummary(summary);
    } catch (e) {
      setStatus(e.message, true);
    }
  }

  async function loadSheriffs() {
    const select = document.getElementById('try-sheriff');
    const run = document.getElementById('try-run');
    try {
      const resp = await getJSON('/api/v1/sheriffs');
      select.replaceChildren();
      const sheriffs = resp.sheriffs || [];
      for (const s of sheriffs) {
        const opt = el('option', s.name);
        opt.value = s.name;
        select.appendChild(opt);
      }
      if (sheriffs.length === 0) {
        select.appendChild(el('option', 'No active sheriffs'));
        setStatus('This instance has no active sheriffs (subscriptions).', true);
        return;
      }
      select.disabled = false;
      run.disabled = false;
      document.getElementById('triage-run').disabled = false;
    } catch (e) {
      select.replaceChildren(el('option', 'Unavailable'));
      setStatus(e.message, true);
    }
  }

  function setUpCopyButtons() {
    for (const button of document.querySelectorAll('button.copy')) {
      button.addEventListener('click', () => {
        const target = document.getElementById(button.dataset.copy);
        if (!target || !navigator.clipboard) {
          return;
        }
        copyText(button, target.textContent);
      });
    }
  }

  function initHome() {
    setUpCopyButtons();
    document.getElementById('try-run').addEventListener('click', runSummary);
    document.getElementById('triage-run').addEventListener('click', runTriage);
    loadSheriffs();
  }

  // ---------------------------------------------------------------------------------------------
  // Trace viewer page.

  const SVG_NS = 'http://www.w3.org/2000/svg';

  /** Returns a new SVG element. Attribute values are numbers or fixed strings, never API data. */
  function svgEl(tag, attrs, className) {
    const e = document.createElementNS(SVG_NS, tag);
    for (const [name, value] of Object.entries(attrs || {})) {
      e.setAttribute(name, String(value));
    }
    if (className) {
      e.setAttribute('class', className);
    }
    return e;
  }

  /** Returns count evenly spaced values from min to max. */
  function ticks(min, max, count) {
    const ret = [];
    for (let i = 0; i < count; i++) {
      ret.push(min + ((max - min) * i) / (count - 1));
    }
    return ret;
  }

  /** Shows the headline of the anomaly as the page heading and title. */
  function setTitle(headline) {
    document.getElementById('trace-title').textContent = headline || 'Trace of an anomaly';
    document.title = (headline ? headline + ' \u00b7 ' : '') + 'perfinsights trace viewer';
  }

  /** Draws the trace of an anomaly: its measurements, commit range and medians. */
  function renderTrace(resp) {
    const out = document.getElementById('trace-result');
    out.replaceChildren();
    const a = resp.anomaly || {};
    const cr = a.commit_range || null;
    const points = (resp.points || []).filter(
      (p) => Number.isFinite(p.commit) && Number.isFinite(p.value)
    );
    setTitle(a.headline);

    const links = el('div', null, 'trace-links');
    if (a.links && a.links.report) {
      links.appendChild(link(a.links.report, 'Report in Perf', 'button-link'));
    }
    if (a.links && a.links.graph) {
      links.appendChild(link(a.links.graph, 'Graph in Perf', 'button-link'));
    }
    out.appendChild(links);

    const meta = el('p', null, 'muted');
    meta.appendChild(
      document.createTextNode(
        'Commits ' +
          resp.begin +
          '\u2013' +
          resp.end +
          ' \u00b7 ' +
          points.length +
          ' points \u00b7 '
      )
    );
    meta.appendChild(el('code', resp.trace_key));
    out.appendChild(meta);
    if (resp.other_traces) {
      out.appendChild(
        el(
          'p',
          'This anomaly covers ' +
            resp.other_traces +
            ' more traces; the chart shows the first one.',
          'muted'
        )
      );
    }
    if (points.length === 0) {
      out.appendChild(el('p', 'No measurements in this commit range.', 'muted'));
      return;
    }

    const W = 960;
    const H = 380;
    const M = { top: 16, right: 20, bottom: 40, left: 72 };
    const x0 = resp.begin;
    const x1 = Math.max(resp.end, resp.begin + 1);
    const values = points.map((p) => p.value);
    const hasMedians =
      Number.isFinite(a.median_before) &&
      Number.isFinite(a.median_after) &&
      (a.median_before !== 0 || a.median_after !== 0);
    if (hasMedians) {
      values.push(a.median_before, a.median_after);
    }
    let y0 = Math.min(...values);
    let y1 = Math.max(...values);
    if (y0 === y1) {
      y0 -= 1;
      y1 += 1;
    }
    const pad = (y1 - y0) * 0.08;
    y0 -= pad;
    y1 += pad;
    const sx = (c) => M.left + ((c - x0) / (x1 - x0)) * (W - M.left - M.right);
    const sy = (v) => H - M.bottom - ((v - y0) / (y1 - y0)) * (H - M.top - M.bottom);

    const svg = svgEl('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img' }, 'trace-chart');
    svg.appendChild(svgEl('title')).textContent = a.headline || 'Trace';

    if (cr) {
      const bx0 = sx(cr.start - 0.5);
      const bx1 = sx(cr.end + 0.5);
      svg.appendChild(
        svgEl(
          'rect',
          {
            x: bx0.toFixed(1),
            y: M.top,
            width: Math.max(3, bx1 - bx0).toFixed(1),
            height: H - M.top - M.bottom,
          },
          'trace-band'
        )
      );
    }
    for (const t of ticks(y0, y1, 5)) {
      const y = sy(t).toFixed(1);
      svg.appendChild(svgEl('line', { x1: M.left, x2: W - M.right, y1: y, y2: y }, 'trace-grid'));
      const label = svgEl(
        'text',
        { x: M.left - 8, y: Number(y) + 4, 'text-anchor': 'end' },
        'trace-tick'
      );
      label.textContent = formatValue(t);
      svg.appendChild(label);
    }
    for (const t of ticks(x0, x1, 6)) {
      const label = svgEl(
        'text',
        {
          x: sx(t).toFixed(1),
          y: H - M.bottom + 20,
          'text-anchor': 'middle',
        },
        'trace-tick'
      );
      label.textContent = String(Math.round(t));
      svg.appendChild(label);
    }
    const axisLabel = svgEl(
      'text',
      { x: W - M.right, y: H - 4, 'text-anchor': 'end' },
      'trace-tick'
    );
    axisLabel.textContent = 'commit';
    svg.appendChild(axisLabel);
    svg.appendChild(
      svgEl('line', { x1: M.left, x2: M.left, y1: M.top, y2: H - M.bottom }, 'trace-axis')
    );
    svg.appendChild(
      svgEl(
        'line',
        {
          x1: M.left,
          x2: W - M.right,
          y1: H - M.bottom,
          y2: H - M.bottom,
        },
        'trace-axis'
      )
    );

    if (hasMedians && cr) {
      const yb = sy(a.median_before).toFixed(1);
      const ya = sy(a.median_after).toFixed(1);
      svg.appendChild(
        svgEl(
          'line',
          {
            x1: sx(x0).toFixed(1),
            x2: sx(cr.start - 0.5).toFixed(1),
            y1: yb,
            y2: yb,
          },
          'trace-median'
        )
      );
      svg.appendChild(
        svgEl(
          'line',
          {
            x1: sx(cr.end + 0.5).toFixed(1),
            x2: sx(x1).toFixed(1),
            y1: ya,
            y2: ya,
          },
          'trace-median'
        )
      );
    }

    const path = points
      .map((p, i) => (i === 0 ? 'M' : 'L') + sx(p.commit).toFixed(1) + ' ' + sy(p.value).toFixed(1))
      .join(' ');
    svg.appendChild(svgEl('path', { d: path }, 'trace-line'));
    const unit = a.test && a.test.unit ? ' ' + a.test.unit : '';
    for (const p of points) {
      const inRange = cr && p.commit >= cr.start && p.commit <= cr.end;
      const dot = svgEl(
        'circle',
        {
          cx: sx(p.commit).toFixed(1),
          cy: sy(p.value).toFixed(1),
          r: inRange ? 4 : 2.5,
        },
        inRange ? 'trace-point trace-point-step' : 'trace-point'
      );
      dot.appendChild(svgEl('title')).textContent =
        'commit ' + p.commit + ': ' + formatValue(p.value) + unit;
      svg.appendChild(dot);
    }
    out.appendChild(svg);

    const legend = el('div', null, 'trace-legend');
    legend.appendChild(el('span', 'commit range of the anomaly', 'legend-band'));
    if (hasMedians) {
      legend.appendChild(el('span', 'median before / after', 'legend-median'));
    }
    legend.appendChild(el('span', 'hover a point to see its commit and value', 'muted'));
    out.appendChild(legend);
  }

  /** Returns a card with a title and an empty key/value table. */
  function detailsCard(title) {
    const card = el('div', null, 'card detail-card');
    card.appendChild(el('h3', title));
    const table = el('table', null, 'kv');
    const rows = el('tbody');
    table.appendChild(rows);
    card.appendChild(table);
    return { card: card, rows: rows };
  }

  /** Adds a row to a key/value table, unless value is empty. value is text or an element. */
  function addRow(rows, label, value) {
    if (value === undefined || value === null || value === '') {
      return;
    }
    const tr = el('tr');
    tr.appendChild(el('th', label));
    const td = el('td');
    if (typeof value === 'object') {
      td.appendChild(value);
    } else {
      td.textContent = String(value);
    }
    tr.appendChild(td);
    rows.appendChild(tr);
  }

  /** Returns a <code> element, or '' (no row) when text is empty. */
  function codeOrEmpty(text) {
    return text ? el('code', text) : '';
  }

  function triageText(triage) {
    const parts = [triage.status || 'unknown'];
    if (triage.bug_ids && triage.bug_ids.length) {
      parts.push('bugs ' + triage.bug_ids.join(', '));
    }
    if (triage.message) {
      parts.push(triage.message);
    }
    return parts.join(' \u00b7 ');
  }

  /** Lists the commits in the anomaly's commit range: the suspects. */
  function renderCommits(out, commits) {
    out.appendChild(el('h2', 'Suspect commits'));
    const rows = (commits && commits.in_range) || [];
    if (rows.length === 0) {
      out.appendChild(el('p', 'The commits of this range are not available.', 'muted'));
      return;
    }
    let note =
      rows.length === 1
        ? 'The only commit in the commit range of the anomaly.'
        : 'The ' + rows.length + ' commits in the commit range of the anomaly.';
    if (commits.truncated) {
      note =
        'The last ' +
        rows.length +
        ' of the ' +
        commits.total +
        ' commits in the commit range of the anomaly.';
    }
    out.appendChild(el('p', note, 'muted'));
    const table = el('table', null, 'tools commits');
    const head = el('tr');
    for (const h of ['Commit', 'Hash', 'Author', 'Subject', 'Time']) {
      head.appendChild(el('th', h));
    }
    table.appendChild(el('thead')).appendChild(head);
    const body = el('tbody');
    for (const c of rows) {
      const tr = el('tr');
      tr.appendChild(el('td', c.number));
      const hash = el('td', null, 'nowrap');
      const short = String(c.hash || '').slice(0, 12);
      hash.appendChild(c.url ? link(c.url, short) : el('code', short));
      tr.appendChild(hash);
      tr.appendChild(el('td', c.author || ''));
      tr.appendChild(el('td', c.subject || ''));
      tr.appendChild(el('td', formatTime(c.ts), 'nowrap'));
      body.appendChild(tr);
    }
    table.appendChild(body);
    const wrap = el('div', null, 'table-wrap');
    wrap.appendChild(table);
    out.appendChild(wrap);
  }

  /** Shows the full view of an anomaly: the change, the test, the alert and the suspects. */
  function renderDetails(d, traceResp) {
    const out = document.getElementById('trace-details');
    out.replaceChildren();
    const test = d.test || {};
    const unit = test.unit ? ' ' + test.unit : '';
    const cr = d.commit_range || {};

    const grid = el('div', null, 'detail-grid');
    const change = detailsCard('The change');
    addRow(
      change.rows,
      'Direction',
      el('span', d.direction, d.direction === 'improvement' ? 'good' : 'bad')
    );
    addRow(change.rows, 'Change', formatPct(d.pct_change));
    addRow(
      change.rows,
      'Median',
      formatValue(d.median_before) + ' \u2192 ' + formatValue(d.median_after) + unit
    );
    addRow(
      change.rows,
      'Commit range',
      cr.start === cr.end ? String(cr.start) : cr.start + '\u2013' + cr.end
    );
    addRow(change.rows, 'Detected', formatTime(d.detected_at));
    addRow(change.rows, 'Sheriff', d.sheriff);
    addRow(change.rows, 'Triage', triageText(d.triage || {}));
    grid.appendChild(change.card);

    const testCard = detailsCard('The test');
    addRow(testCard.rows, 'Test path', codeOrEmpty(test.test_path));
    addRow(testCard.rows, 'Bot', test.bot);
    addRow(testCard.rows, 'Benchmark', test.benchmark);
    addRow(testCard.rows, 'Test', test.test);
    addRow(testCard.rows, 'Subtests', (test.subtests || []).join(' / '));
    addRow(testCard.rows, 'Unit', test.unit);
    addRow(
      testCard.rows,
      'Traces',
      d.num_traces > 1 ? d.num_traces + ' (the chart shows the first)' : '1'
    );
    grid.appendChild(testCard.card);

    const alert = d.alert;
    if (alert) {
      const why = detailsCard('Why it was flagged');
      addRow(why.rows, 'Alert', alert.display_name || 'alert ' + alert.id);
      addRow(
        why.rows,
        'Detection',
        [
          alert.algo,
          alert.step ? 'step ' + alert.step : '',
          'radius ' + alert.radius,
          'threshold ' + alert.threshold,
        ]
          .filter(Boolean)
          .join(', ')
      );
      addRow(why.rows, 'Direction', alert.direction);
      addRow(why.rows, 'Query', codeOrEmpty(alert.query));
      addRow(why.rows, 'Group by', alert.group_by);
      addRow(why.rows, 'Minimum traces', alert.minimum_num ? String(alert.minimum_num) : '');
      addRow(why.rows, 'Action', alert.action);
      grid.appendChild(why.card);
    }
    out.appendChild(grid);

    renderCommits(out, d.commits);

    const others = d.other_traces || [];
    if (others.length) {
      const box = el('details', null, 'raw');
      box.appendChild(el('summary', 'The other ' + others.length + ' traces of this anomaly'));
      const list = el('ul', null, 'trace-keys');
      for (const key of others) {
        list.appendChild(el('li')).appendChild(el('code', key));
      }
      box.appendChild(list);
      out.appendChild(box);
    }

    const raw = el('details', null, 'raw');
    raw.appendChild(el('summary', 'Raw JSON (what an agent receives)'));
    raw.appendChild(el('p', 'GET /api/v1/anomalies/' + d.id, 'muted'));
    raw.appendChild(el('pre', JSON.stringify(d, null, 2)));
    raw.appendChild(el('p', 'GET /api/v1/anomalies/' + d.id + '/trace', 'muted'));
    raw.appendChild(el('pre', JSON.stringify(traceResp, null, 2)));
    out.appendChild(raw);
  }

  /** Counts requests, so a slow response can't replace the one the user asked for last. */
  let traceRequest = 0;

  /** Loads and shows the trace and details of an anomaly, and keeps them in the page URL. */
  async function showAnomaly(rawId) {
    const id = String(rawId || '').trim();
    const result = document.getElementById('trace-result');
    const details = document.getElementById('trace-details');
    if (!id) {
      setStatusIn('trace-status', 'Enter an anomaly id.', true);
      return;
    }
    const before = document.getElementById('trace-before').value;
    const after = document.getElementById('trace-after').value;
    if (window.history && window.history.replaceState) {
      const q = new URLSearchParams({ id: id, before: before, after: after });
      window.history.replaceState(null, '', window.location.pathname + '?' + q.toString());
    }
    const request = ++traceRequest;
    setStatusIn('trace-status', 'Loading\u2026');
    const path = '/api/v1/anomalies/' + encodeURIComponent(id);
    const params = new URLSearchParams({ commits_before: before, commits_after: after });
    const [trace, detail] = await Promise.allSettled([
      getJSON(path + '/trace?' + params.toString()),
      getJSON(path),
    ]);
    if (request !== traceRequest) {
      return;
    }
    if (trace.status === 'rejected') {
      result.replaceChildren();
      details.replaceChildren();
      setTitle('');
      setStatusIn('trace-status', trace.reason.message, true);
      return;
    }
    setStatusIn('trace-status', '');
    renderTrace(trace.value);
    if (detail.status === 'fulfilled') {
      renderDetails(detail.value, trace.value);
    } else {
      details.replaceChildren(
        el('p', 'Could not load the details: ' + detail.reason.message, 'muted')
      );
    }
  }

  /** Selects the option of a <select> with the given value, if it has one. */
  function selectOption(id, value) {
    const select = document.getElementById(id);
    if (!value || !select.options) {
      return;
    }
    for (const opt of select.options) {
      if (opt.value === value) {
        select.value = value;
        return;
      }
    }
  }

  function initTrace() {
    const params = new URLSearchParams(window.location.search);
    selectOption('trace-before', params.get('before'));
    selectOption('trace-after', params.get('after'));
    const input = document.getElementById('trace-id');
    input.value = (params.get('id') || '').trim();
    const show = () => showAnomaly(input.value);
    document.getElementById('trace-run').addEventListener('click', show);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        show();
      }
    });
    for (const id of ['trace-before', 'trace-after']) {
      document.getElementById(id).addEventListener('change', () => {
        if (input.value.trim()) {
          show();
        }
      });
    }
    if (input.value) {
      show();
    } else {
      setStatusIn('trace-status', 'Enter an anomaly id, or open this page from a Trace link.');
    }
  }

  document.addEventListener('DOMContentLoaded', () => {
    BASE = readBase();
    if (document.body && document.body.dataset && document.body.dataset.page === 'trace') {
      initTrace();
    } else {
      initHome();
    }
  });
})();

// Mock Data based on requirements
const mockData = {
  builders: [
    {
      id: 'builder-android-arm',
      name: 'Android ARM Builder',
      pool: 'luci.chrome.perf.ci',
      testerCoordinators: [
        {
          id: 'tc-pixel9',
          name: 'Pixel 9 Coordinator',
          pool: 'luci.chrome.perf.ci',
          status: 'pass',
          benchmarks: ['system_health.common_mobile', 'v8.browsing_mobile'],
          testMachines: [
            { id: 'machine-p9-1', pool: 'chrome.tests.perf', status: 'up' },
            { id: 'machine-p9-2', pool: 'chrome.tests.perf', status: 'down' },
            { id: 'machine-p9-3', pool: 'chrome.tests.perf', status: 'up' },
          ],
        },
      ],
    },
  ],
};

class AsyncQueue {
  constructor(concurrency) {
    this.concurrency = concurrency;
    this.running = 0;
    this.queue = [];
  }

  add(task) {
    return new Promise((resolve, reject) => {
      this.queue.push(async () => {
        try {
          resolve(await task());
        } catch (e) {
          reject(e);
        }
      });
      this.runNext();
    });
  }

  runNext() {
    if (this.running >= this.concurrency || this.queue.length === 0) return;
    this.running++;
    const nextTask = this.queue.shift();
    nextTask().finally(() => {
      this.running--;
      this.runNext();
    });
  }
}

const swarmingQueue = new AsyncQueue(3);

document.addEventListener('DOMContentLoaded', () => {
  initDashboard();

  // Auto-refresh the dashboard every 5 minutes
  setInterval(
    () => {
      console.log('Auto-refreshing dashboard in background...');
      refreshDashboardGracefully();
    },
    5 * 60 * 1000
  );
});

async function refreshDashboardGracefully() {
  let overlay = document.getElementById('refresh-overlay');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.id = 'refresh-overlay';
    overlay.style.position = 'fixed';
    overlay.style.top = '20px';
    overlay.style.right = '20px';
    overlay.style.background = 'rgba(15, 23, 42, 0.9)';
    overlay.style.border = '1px solid rgba(255, 255, 255, 0.1)';
    overlay.style.backdropFilter = 'blur(10px)';
    overlay.style.padding = '12px 16px';
    overlay.style.borderRadius = '8px';
    overlay.style.color = 'var(--text-primary)';
    overlay.style.fontSize = '0.85rem';
    overlay.style.zIndex = '9999';
    overlay.style.display = 'flex';
    overlay.style.flexDirection = 'column';
    overlay.style.gap = '8px';
    overlay.style.boxShadow =
      '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)';
    document.body.appendChild(overlay);
  }

  const steps = [
    'Fetching Buildbucket...',
    'Fetching Starlark triggers...',
    'Fetching GN Args...',
    'Fetching Swarming configs...',
    'Fetching Benchmarks mapping...',
    'Parsing data...',
  ];

  overlay.innerHTML = `
        <div style="display: flex; align-items: center; gap: 8px;">
            <div class="spinner" style="width: 14px; height: 14px; border-width: 2px;"></div>
            <span style="font-weight: 600;">Refreshing Data...</span>
        </div>
        <div id="refresh-step-text" style="color: var(--text-secondary); font-size: 0.75rem;">Starting...</div>
    `;
  overlay.style.display = 'flex';

  try {
    let lastStepTime = performance.now();
    const onProgress = (stepIndex) => {
      const now = performance.now();
      const elapsed = ((now - lastStepTime) / 1000).toFixed(1) + 's';
      lastStepTime = now;

      const stepTextEl = document.getElementById('refresh-step-text');
      if (stepTextEl && stepIndex < steps.length) {
        stepTextEl.innerText = `${steps[stepIndex]} (last: ${elapsed})`;
      }
    };

    // Save currently expanded rows
    const expandedRows = Array.from(document.querySelectorAll('.builder-row.expanded')).map(
      (tr) => {
        return tr.querySelector('.builder-name').innerText;
      }
    );

    let result = await fetchBuilders(onProgress);

    if (result.builders.length > 0) {
      renderDashboard(result.builders, result.gnArgsMapping, result.swarmingDimensionsMapping);

      // Restore expanded state
      document.querySelectorAll('.builder-row').forEach((tr) => {
        const nameEl = tr.querySelector('.builder-name');
        if (nameEl && expandedRows.includes(nameEl.innerText)) {
          const btn = tr.querySelector('.toggle-btn');
          if (btn) btn.click();
        }
      });

      // Re-apply search filter
      const searchInput = document.getElementById('searchInput');
      if (searchInput) searchInput.dispatchEvent(new Event('input'));
    }

    overlay.innerHTML = `
            <div style="display: flex; align-items: center; gap: 8px;">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--status-green)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                <span style="font-weight: 600;">Refresh Complete</span>
            </div>
        `;
    setTimeout(() => {
      overlay.style.display = 'none';
    }, 3000);
  } catch (e) {
    overlay.innerHTML = `
            <div style="display: flex; align-items: center; gap: 8px; color: var(--status-red);">
                <span style="font-weight: 600;">Refresh Failed</span>
            </div>
        `;
    setTimeout(() => {
      overlay.style.display = 'none';
    }, 3000);
  }
}

async function fetchBuilders(onProgress) {
  try {
    if (onProgress) onProgress(0);
    const res1 = await fetch('/api/builders');
    if (!res1.ok) throw new Error(`Buildbucket Status: ${res1.status}`);
    const data1 = await res1.json();

    if (onProgress) onProgress(1);
    const res2 = await fetch('/api/trigger_mapping');
    if (!res2.ok) throw new Error(`Starlark Status: ${res2.status}`);
    const triggerMapping = await res2.json();

    if (onProgress) onProgress(2);
    const res3 = await fetch('/api/gn_args');
    if (!res3.ok) throw new Error(`GN Args Status: ${res3.status}`);
    const gnArgsMapping = await res3.json();

    if (onProgress) onProgress(3);
    const res4 = await fetch('/api/swarming_dimensions');
    if (!res4.ok) throw new Error(`Swarming config Status: ${res4.status}`);
    const swarmingDimensionsMapping = await res4.json();

    if (onProgress) onProgress(4);
    const res5 = await fetch('/api/benchmarks');
    if (!res5.ok) throw new Error(`Benchmarks Status: ${res5.status}`);
    const benchmarksMapping = await res5.json();

    if (onProgress) onProgress(5);
    const regex = /.*-perf(-pgo)?$/;
    const IGNORE_PATTERNS = ['-processor-perf', 'media-perf', 'mediarouter-perf', 'fuchsia'];
    const perfAll = (data1.builders || []).filter((b) => {
      if (!regex.test(b.id.builder)) return false;
      if (IGNORE_PATTERNS.some((p) => b.id.builder.includes(p))) return false;
      return true;
    });

    const buildersList = perfAll.filter((b) => b.id.builder.includes('builder'));
    const testersList = perfAll.filter((b) => !b.id.builder.includes('builder'));

    let mappedBuilders = buildersList.map((b) => {
      return {
        id: b.id.builder,
        name: b.id.builder,
        pool: 'luci.chrome.perf.ci',
        testerCoordinators: [],
      };
    });

    // Unmapped container
    let unmapped = {
      id: 'unmapped',
      name: 'Unmapped Testers',
      pool: 'luci.chrome.perf.ci',
      testerCoordinators: [],
    };

    testersList.forEach((t) => {
      const swarmingDims = swarmingDimensionsMapping[t.id.builder] || {};
      const pool = swarmingDims.pool || 'Unknown Pool';

      const tc = {
        id: t.id.builder,
        name: t.id.builder,
        pool: pool,
        status: 'running',
        testMachines: [], // to be filled later by swarming async
        benchmarks: benchmarksMapping[t.id.builder] || [],
      };

      const parentBuilder = triggerMapping[tc.id];
      if (parentBuilder) {
        const parent = mappedBuilders.find((b) => b.id === parentBuilder);
        if (parent) {
          parent.testerCoordinators.push(tc);
        } else {
          unmapped.testerCoordinators.push(tc);
        }
      } else {
        unmapped.testerCoordinators.push(tc);
      }
    });

    if (unmapped.testerCoordinators.length > 0) {
      mappedBuilders.push(unmapped);
    }

    // Sort builders alphabetically for UI consistency, putting Unmapped at the bottom
    mappedBuilders.sort((a, b) => {
      if (a.id === 'unmapped') return 1;
      if (b.id === 'unmapped') return -1;
      return a.name.localeCompare(b.name);
    });

    if (onProgress) onProgress(6);
    return { builders: mappedBuilders, gnArgsMapping, swarmingDimensionsMapping };
  } catch (e) {
    console.error('Error fetching builders:', e);
    throw e;
  }
}

async function initDashboard() {
  const steps = [
    { id: 'step-0', text: 'Fetching Buildbucket...' },
    { id: 'step-1', text: 'Fetching Starlark triggers...' },
    { id: 'step-2', text: 'Fetching GN Args...' },
    { id: 'step-3', text: 'Fetching Swarming configs...' },
    { id: 'step-4', text: 'Fetching Benchmarks mapping...' },
    { id: 'step-5', text: 'Parsing data...' },
  ];

  let checklistHtml = steps
    .map(
      (s) => `
        <div id="${s.id}" style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px; color: var(--text-secondary); opacity: 0.5; transition: all 0.3s ease;">
            <div class="step-icon" style="width: 24px; height: 24px; display: flex; align-items: center; justify-content: center;">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.3"><circle cx="12" cy="12" r="10"/></svg>
            </div>
            <span class="step-text" style="font-size: 0.95rem; font-weight: 500;">${s.text}</span>
        </div>
    `
    )
    .join('');

  document.getElementById('dashboard-content').innerHTML = `
        <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; height: 400px;">
            <div class="spinner" style="margin-bottom: 30px;"></div>
            <div style="display: flex; flex-direction: column; align-items: flex-start;">
                ${checklistHtml}
            </div>
        </div>
    `;

  try {
    let lastStepTime = performance.now();
    const onProgress = (stepIndex) => {
      const now = performance.now();
      const elapsed = ((now - lastStepTime) / 1000).toFixed(1) + 's';
      lastStepTime = now;

      if (stepIndex > 0 && stepIndex <= steps.length) {
        // Mark previous step as done
        const prevEl = document.getElementById(`step-${stepIndex - 1}`);
        if (prevEl) {
          prevEl.style.color = 'var(--text-primary)';
          prevEl.style.opacity = '1';
          const iconContainer = prevEl.querySelector('.step-icon');
          iconContainer.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--status-green)" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;
          const textSpan = prevEl.querySelector('.step-text');
          if (textSpan && !textSpan.innerHTML.includes('s)')) {
            textSpan.innerHTML += ` <span style="color:var(--text-tertiary); font-size:0.8rem; margin-left:6px;">(${elapsed})</span>`;
          }
        }
      }
      if (stepIndex < steps.length) {
        // Mark current step as active
        const curEl = document.getElementById(`step-${stepIndex}`);
        if (curEl) {
          curEl.style.color = 'var(--accent-primary)';
          curEl.style.opacity = '1';
          const iconContainer = curEl.querySelector('.step-icon');
          iconContainer.innerHTML = `<div class="spinner" style="width:20px;height:20px;border-width:2px;"></div>`;
        }
      }
    };

    let result = await fetchBuilders(onProgress);
    let builders = result.builders;
    let gnArgsMapping = result.gnArgsMapping;
    let swarmingDimensionsMapping = result.swarmingDimensionsMapping;

    if (builders.length === 0) {
      document.getElementById('dashboard-content').innerHTML =
        '<div style="color: var(--status-red);">No builders found matching the format .*-builder-perf in chrome/ci bucket. Check console for details.</div>';
      return;
    }

    renderDashboard(builders, gnArgsMapping, swarmingDimensionsMapping);
  } catch (e) {
    document.getElementById('dashboard-content').innerHTML =
      `<div style="color: var(--status-red);">Error fetching from Buildbucket: ${e.message}</div>`;
  }
}

function renderDashboard(builders, gnArgsMapping = {}, swarmingDimensionsMapping = {}) {
  const container = document.getElementById('dashboard-content');

  let html = `
        <table class="glass-table">
            <thead>
                <tr>
                    <th style="width: 40px;"></th>
                    <th style="width: 35%;">Builder / Tester Name</th>
                    <th style="width: 15%;">Pool</th>
                    <th style="width: 30%;">Swarming Dimensions</th>
                    <th>Devices (Total | Q/D)</th>
                </tr>
            </thead>
            <tbody id="table-body">
    `;

  container.innerHTML = html + '</tbody></table>';
  const tbody = document.getElementById('table-body');

  builders.forEach((builder) => {
    // Builder Row
    const tr = document.createElement('tr');
    tr.className = 'builder-row';
    tr.setAttribute('data-builder-id', builder.id);
    tr.setAttribute('data-search', builder.name.toLowerCase());
    tr.innerHTML = `
            <td class="toggle-cell">
                <button class="toggle-btn" aria-expanded="false">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="9 18 15 12 9 6"></polyline>
                    </svg>
                </button>
            </td>
            <td class="name-cell">
                <div style="display: flex; align-items: center;">
                    <span class="builder-name" style="font-size: 1.1rem; font-weight: 600; color: #f8fafc;">
                        ${builder.id !== 'unmapped' ? `<a href="https://ci.chromium.org/ui/p/chrome/builders/luci.chrome.ci/${builder.name}" target="_blank" style="color: inherit; text-decoration: none; border-bottom: 1px dotted rgba(255,255,255,0.3); transition: border-color 0.2s;" onmouseover="this.style.borderColor='rgba(255,255,255,0.8)'" onmouseout="this.style.borderColor='rgba(255,255,255,0.3)'">${builder.name}</a>` : builder.name}
                    </span>
                    ${builder.id !== 'unmapped' ? `<button class="gn-args-toggle" style="margin-left: 10px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); color: #94a3b8; border-radius: 4px; padding: 2px 8px; font-size: 0.75rem; cursor: pointer; transition: 0.2s;">GN Args</button>` : ''}
                </div>
            </td>
            <td><span class="pool-tag">${builder.pool}</span></td>
            <td></td>
            <td></td>
        `;
    tbody.appendChild(tr);

    // GN Args Row
    const gnArgs = gnArgsMapping[builder.id] || 'No GN Args mapping found in mb_config.pyl';
    const gnRow = document.createElement('tr');
    gnRow.className = 'gn-args-container';
    gnRow.style.display = 'none';
    gnRow.innerHTML = `
            <td></td>
            <td colspan="4" style="padding: 0 1rem 1rem 1rem;">
                <div style="background: rgba(0,0,0,0.4); border: 1px solid rgba(255,255,255,0.1); border-radius: 4px; padding: 12px; font-family: monospace; font-size: 0.85rem; color: #a855f7; white-space: pre-wrap; word-break: break-all;">${gnArgs}</div>
            </td>
        `;
    tbody.appendChild(gnRow);

    const gnBtn = tr.querySelector('.gn-args-toggle');
    if (gnBtn) {
      gnBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        const isExpanded = gnRow.style.display !== 'none';
        gnRow.style.display = isExpanded ? 'none' : 'table-row';
        gnBtn.style.color = isExpanded ? '#94a3b8' : '#fff';
        gnBtn.style.background = isExpanded ? 'rgba(255,255,255,0.05)' : 'var(--accent-primary)';
      });
    }

    // Testers Container Row
    const tcContainerTr = document.createElement('tr');
    tcContainerTr.className = 'testers-container';
    tcContainerTr.style.display = 'none';

    const tcTd = document.createElement('td');
    tcTd.colSpan = 5;
    tcTd.style.padding = '0';

    let tcHtml = '<table class="tester-table"><tbody>';

    if (builder.testerCoordinators && builder.testerCoordinators.length > 0) {
      builder.testerCoordinators.forEach((tc) => {
        const statusClass = `status-${tc.status}`;
        const statusText = tc.status.replace('_', ' ');
        const safeTcName = tc.name.replace(/[^a-zA-Z0-9-]/g, '_');

        const dims = swarmingDimensionsMapping[tc.name] || {};
        const dimsHtml =
          Object.entries(dims)
            .map(
              ([k, v]) =>
                `<div style="font-size: 0.75rem; color: #94a3b8; margin-bottom:2px;"><span style="color: #a855f7;">${k}</span>: ${v}</div>`
            )
            .join('') ||
          '<span style="color:var(--text-tertiary);font-size:0.8rem;">No dimensions mapped</span>';

        const dimsSearchStr = Object.values(dims).join(' ').toLowerCase();
        const benchSearchStr = (tc.benchmarks || []).join(' ').toLowerCase();
        const searchStr = `${tc.name.toLowerCase()} ${dimsSearchStr} ${benchSearchStr}`;

        const chromeosTesters = [
          'android-corsola-steelix-8gb-perf',
          'android-brya-kano-i5-8gb-perf',
          'android-nissa-uldren-8gb-perf',
        ];
        let swarmingServer = chromeosTesters.includes(tc.name)
          ? 'https://chromeos-swarming.appspot.com'
          : 'https://chrome-swarming.appspot.com';

        let botLink = `${swarmingServer}/botlist`;
        if (Object.keys(dims).length > 0) {
          botLink +=
            '?f=' +
            Object.entries(dims)
              .map(([k, v]) => encodeURIComponent(`${k}:${v}`))
              .join('&f=');
        }

        let benchHtml =
          tc.benchmarks && tc.benchmarks.length > 0
            ? `<div style="display: flex; flex-wrap: wrap; gap: 4px; margin: 6px 0;">` +
              tc.benchmarks
                .map(
                  (b) =>
                    `<div class="benchmark-badge" style="font-size: 0.7rem; background: rgba(56, 189, 248, 0.05); border: 1px solid rgba(56, 189, 248, 0.15); color: #38bdf8; border-radius: 4px; padding: 2px 6px; white-space: nowrap;">${b}</div>`
                )
                .join('') +
              `</div>`
            : '';

        tcHtml += `
                    <tr class="tester-row" data-search="${searchStr}">
                        <td style="width: 40px; text-align: center; vertical-align: top; padding-top: 1rem;">
                            <div class="tree-line"></div>
                        </td>
                        <td style="width: 35%; vertical-align: top; padding-top: 1rem;">
                            <div style="font-weight: 500; color: #e2e8f0; margin-bottom: 4px;">
                                <a href="https://ci.chromium.org/ui/p/chrome/builders/luci.chrome.ci/${tc.name}" target="_blank" style="color: inherit; text-decoration: none; border-bottom: 1px dotted rgba(255,255,255,0.3); transition: border-color 0.2s;" onmouseover="this.style.borderColor='rgba(255,255,255,0.8)'" onmouseout="this.style.borderColor='rgba(255,255,255,0.3)'">${tc.name}</a>
                            </div>
                            ${benchHtml}
                            <div style="font-size: 0.75rem; color: #94a3b8; display: flex; gap: 4px; flex-wrap: wrap;">
                                <span class="status-badge ${statusClass}">${statusText}</span>
                            </div>
                        </td>
                        <td style="width: 15%; vertical-align: top; padding-top: 1rem;">
                            <span class="pool-tag" style="background: rgba(255,255,255,0.05); color: #94a3b8; border: none;">${tc.pool}</span>
                        </td>
                        <td style="width: 30%; vertical-align: top; padding-top: 1rem;">
                            ${dimsHtml}
                        </td>
                        <td style="vertical-align: top; padding-top: 1rem;">
                            <div id="swarming-data-${safeTcName}">
                                <div class="spinner" style="width:20px;height:20px;border-width:2px;display:inline-block;"></div>
                            </div>
                            <a href="${botLink}" target="_blank" style="font-size: 0.75rem; color: var(--accent-primary); text-decoration: none; margin-top: 6px; display: inline-block;">View in Swarming &#x2197;</a>
                        </td>
                    </tr>
                `;

        // Fetch swarming data for this tc asynchronously via the queue
        if (Object.keys(dims).length > 0) {
          swarmingQueue
            .add(() =>
              fetch('/api/swarming_bots', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ dimensions: dims, server: swarmingServer }),
              }).then((r) => r.json())
            )
            .then((botData) => {
              const items = botData.items || [];
              const total = items.length;
              const dead = items.filter((b) => b.isDead).length;
              const quarantined = items.filter((b) => b.quarantined).length;
              const qd = items.filter((b) => b.isDead || b.quarantined).length;

              let bugUrl = '#';
              if (qd > 0) {
                const deadBotsList = items
                  .filter((b) => b.isDead || b.quarantined)
                  .map((b) => b.botId);
                const deadBotsMarkdownList = deadBotsList.map(
                  (botId) => `[${botId}](${swarmingServer}/bot?id=${botId})`
                );

                const title = encodeURIComponent(`[Browser][Repair][${qd}] - ${tc.name}`);
                const ccList = encodeURIComponent(
                  'jeffyoon@google.com,flops-repairs@google.com,sslobodow@google.com'
                );
                const description =
                  encodeURIComponent(`**Requester: Instructions on how to create a bug: go/flops-browser-escalations/. Your request will automatically be placed in work queue for repair. Please do not explicitly assign bugs to individuals without prior discussion with said individuals. Reminder to update both title and description.**

---

**Hostnames:**
${deadBotsMarkdownList.join('\n')}

**Pool(s):**
${tc.pool}

**Issue / Request:**
Machines are reported as quarantined or dead in Swarming.

**Logs & Swarming link:**
${botLink}

---------------------

**Additional Info (Optional):**  `);

                bugUrl = `https://issuetracker.google.com/issues/new?component=1735976&template=2161122&title=${title}&description=${description}&cc=${ccList}&priority=P1&type=CUSTOMER_ISSUE`;
              }

              const container = document.getElementById(`swarming-data-${safeTcName}`);
              if (container) {
                let textHtml = `<div style="font-size: 0.9rem; font-weight: 600; color: #e2e8f0; margin-bottom:4px;">${total} total</div>`;
                if (qd > 0) {
                  textHtml += `<div style="font-size: 0.8rem; color: var(--status-red);">${qd} Q/D</div>`;
                  textHtml += `<a href="${bugUrl}" target="_blank" style="font-size: 0.75rem; color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.1); border-radius: 4px; padding: 2px 6px; text-decoration: none; display: inline-block; margin-top: 4px;">File Bug</a>`;
                } else {
                  textHtml += `<div style="font-size: 0.8rem; color: var(--status-green);">0 Q/D</div>`;
                }
                container.innerHTML = textHtml;
              }
            })
            .catch((e) => {
              const container = document.getElementById(`swarming-data-${safeTcName}`);
              if (container)
                container.innerHTML = `<span style="color:var(--status-red);font-size:0.8rem;">Error</span>`;
            });
        } else {
          setTimeout(() => {
            const container = document.getElementById(`swarming-data-${safeTcName}`);
            if (container)
              container.innerHTML = `<span style="color:var(--text-tertiary);font-size:0.8rem;">N/A</span>`;
          }, 0);
        }
      });
    } else {
      tcHtml +=
        '<tr><td colspan="5" style="padding: 1rem 1rem 1rem 3.5rem; color: var(--text-tertiary);">No tester coordinators mapped yet.</td></tr>';
    }

    tcHtml += '</tbody></table>';
    tcTd.innerHTML = tcHtml;
    tcContainerTr.appendChild(tcTd);
    tbody.appendChild(tcContainerTr);

    // Toggle Logic
    const btn = tr.querySelector('.toggle-btn');
    btn.addEventListener('click', () => {
      const isExpanded = btn.getAttribute('aria-expanded') === 'true';
      btn.setAttribute('aria-expanded', !isExpanded);
      tcContainerTr.style.display = isExpanded ? 'none' : 'table-row';
      tr.classList.toggle('expanded', !isExpanded);
    });
  });

  initSearch();
}

function initSearch() {
  const searchInput = document.getElementById('searchInput');
  if (searchInput && !searchInput.hasAttribute('data-bound')) {
    searchInput.setAttribute('data-bound', 'true');
    searchInput.addEventListener('input', (e) => {
      const query = e.target.value.toLowerCase().trim();
      const builderRows = document.querySelectorAll('.builder-row');

      builderRows.forEach((bRow) => {
        const bText = bRow.getAttribute('data-search') || '';
        const tContainer = bRow.nextElementSibling.nextElementSibling;
        const tRows = tContainer.querySelectorAll('.tester-row');

        let builderMatches = query === '' || bText.includes(query);
        let anyTesterMatches = false;

        tRows.forEach((tRow) => {
          const tText = tRow.getAttribute('data-search');
          if (tText) {
            const testerMatches = query === '' || builderMatches || tText.includes(query);
            tRow.style.display = testerMatches ? '' : 'none';
            if (testerMatches) anyTesterMatches = true;
          }
        });

        if (builderMatches || anyTesterMatches) {
          bRow.style.display = '';
          if (query !== '' && anyTesterMatches) {
            tContainer.style.display = 'table-row';
            const toggleBtn = bRow.querySelector('.toggle-btn');
            if (toggleBtn) {
              toggleBtn.setAttribute('aria-expanded', 'true');
              bRow.classList.add('expanded');
            }
          } else if (query === '') {
            tContainer.style.display = 'none';
            const toggleBtn = bRow.querySelector('.toggle-btn');
            if (toggleBtn) {
              toggleBtn.setAttribute('aria-expanded', 'false');
              bRow.classList.remove('expanded');
            }
          }
        } else {
          bRow.style.display = 'none';
          tContainer.style.display = 'none';
        }

        const gnRow = bRow.nextElementSibling;
        if (query !== '') {
          gnRow.style.display = 'none';
          const gnBtn = bRow.querySelector('.gn-args-toggle');
          if (gnBtn) {
            gnBtn.style.color = '#94a3b8';
            gnBtn.style.background = 'rgba(255,255,255,0.05)';
          }
        }
      });
    });
  }
}

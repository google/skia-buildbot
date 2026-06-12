import './trace-chart-sk';
import { TraceChartSk } from './trace-chart-sk';
import { expect } from 'chai';

describe('trace-chart-sk', () => {
  let element: TraceChartSk;

  beforeEach(async () => {
    element = document.createElement('trace-chart-sk') as TraceChartSk;
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('renders chart-tooltip-sk when hovered', async () => {
    // Simulate hover by setting the internal state
    (element as any)['_hoveredPoint'] = {
      series: { id: 'test', color: '#fff', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const tooltip = element.shadowRoot!.querySelector('trace-chart-tooltip-sk');
    expect(tooltip).to.not.be.null;
  });

  it('computes subrepo rolls correctly', async () => {
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 1, val: 10.0, createdat: 1000, metadata: { V8: 'v1' } },
          { commit_number: 2, val: 11.0, createdat: 2000, metadata: { V8: 'v2' } },
        ],
      },
    ];
    element.selectedSubrepo = 'V8';
    await element.updateComplete;

    const rolls = (element as any)['_subrepoRolls'];
    expect(rolls.length).to.equal(1);
    expect(rolls[0].oldVer).to.equal('v1');
    expect(rolls[0].newVer).to.equal('v2');
  });

  it('reads CSS variables for chart colors', async () => {
    const oldGetComputedStyle = window.getComputedStyle;
    let called = false;
    window.getComputedStyle = (el: Element) => {
      if (el === element) {
        called = true;
      }
      return oldGetComputedStyle(el);
    };

    try {
      (element as any)['_processedSeries'] = [
        { id: 'test', color: '#fff', rows: [{ commit_number: 1, val: 1, createdat: 1 }] },
      ];
      (element as any)['_drawBackground']();

      expect(called).to.be.true;
    } finally {
      window.getComputedStyle = oldGetComputedStyle;
    }
  });

  it('uses commit numbers by default on X axis', async () => {
    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    const ctx = canvas.getContext('2d')!;
    const oldFillText = ctx.fillText;
    const texts: string[] = [];
    ctx.fillText = function (text: string, x: number, y: number) {
      texts.push(text);
      oldFillText.call(this, text, x, y);
    };

    try {
      (element as any)['_processedSeries'] = [
        {
          id: 'test',
          color: '#fff',
          rows: [{ commit_number: 100, val: 1, createdat: 1234567890 }],
        },
      ];
      (element as any)['_drawBackground']();

      const hasDate = texts.some((t) => t.includes('-'));
      expect(hasDate).to.be.false;

      const hasCommit = texts.some((t) => t.includes('100'));
      expect(hasCommit).to.be.true;
    } finally {
      ctx.fillText = oldFillText;
    }
  });

  it('uses dates on X axis when dateMode is enabled', async () => {
    element.dateMode = true;
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    const ctx = canvas.getContext('2d')!;
    const oldFillText = ctx.fillText;
    const texts: string[] = [];
    ctx.fillText = function (text: string, x: number, y: number) {
      texts.push(text);
      oldFillText.call(this, text, x, y);
    };

    try {
      (element as any)['_processedSeries'] = [
        {
          id: 'test',
          color: '#fff',
          rows: [{ commit_number: 100, val: 1, createdat: 1234567890 }],
        },
      ];
      (element as any)['_drawBackground']();

      const hasDate = texts.some((t) => t.includes('-'));
      expect(hasDate).to.be.true;
    } finally {
      ctx.fillText = oldFillText;
    }
  });

  it('emits range-selected event on Ctrl+Drag', async () => {
    let eventDetail: any = null;
    element.addEventListener('range-selected', (e: any) => {
      eventDetail = e.detail;
    });

    // Setup mapping and dimensions so calculations don't fail
    (element as any)['_processedSeries'] = [
      { id: 'test', color: '#fff', rows: [{ commit_number: 100, val: 1, createdat: 1 }] },
    ];

    // Mock getChartBoundsAndMapping to return expected values
    const oldGetMapping = (element as any)['_getChartBoundsAndMapping'];
    (element as any)['_getChartBoundsAndMapping'] = () => ({
      minX: 0,
      maxX: 1000,
      padding: { left: 0, top: 0, right: 0, bottom: 0 },
      graphWidth: 1000,
      graphHeight: 400,
    });

    try {
      // Simulate pointer down with Ctrl
      (element as any)['_dragCtx'] = {
        isDragging: true,
        dragStartX: 100,
        dragStartY: 100,
        isCtrl: true,
        currentX: 200,
      };

      const upEvent = new PointerEvent('pointerup', { clientX: 200, clientY: 100, ctrlKey: true });
      // Mock canvas.getBoundingClientRect
      const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
      canvas.getBoundingClientRect = () =>
        ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

      (element as any)['_handlePointerUp'](upEvent);

      expect(eventDetail).to.not.be.null;
      expect(eventDetail.startX).to.equal(100);
      expect(eventDetail.endX).to.equal(200);
    } finally {
      (element as any)['_getChartBoundsAndMapping'] = oldGetMapping;
    }
  });

  it('draws No Data message when minX is Infinity', async () => {
    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    const ctx = canvas.getContext('2d')!;
    const oldFillText = ctx.fillText;
    const texts: string[] = [];
    ctx.fillText = function (text: string, x: number, y: number) {
      texts.push(text);
      oldFillText.call(this, text, x, y);
    };

    try {
      (element as any)['_processedSeries'] = [{ id: 'test', color: '#fff', rows: [] }];
      (element as any)['_drawBackground']();

      const hasMessage = texts.some((t) => t.includes('No data available'));
      expect(hasMessage).to.be.true;
    } finally {
      ctx.fillText = oldFillText;
    }
  });

  it('spaces X axis evenly when evenXAxisSpacing is enabled', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
          { commit_number: 100, val: 3, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const mapping = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());

    const x1 = mapping.mapX(10);
    const x2 = mapping.mapX(20);
    const x3 = mapping.mapX(100);

    const diff1 = x2 - x1;
    const diff2 = x3 - x2;

    expect(Math.abs(diff1 - diff2)).to.be.below(1);
  });

  it('generates X axis ticks at indices when evenXAxisSpacing is enabled', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
          { commit_number: 100, val: 3, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    const ctx = canvas.getContext('2d')!;
    const oldFillText = ctx.fillText;
    const texts: string[] = [];
    ctx.fillText = function (text: string, x: number, y: number) {
      texts.push(text);
      oldFillText.call(this, text, x, y);
    };

    try {
      (element as any)['_drawBackground']();

      expect(texts).to.include('10');
      expect(texts).to.include('20');
      expect(texts).to.include('100');
      expect(texts).to.not.include('55');
    } finally {
      ctx.fillText = oldFillText;
    }
  });

  it('zooms correctly in even spacing mode when bounds are not exact matches', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
          { commit_number: 100, val: 3, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    // Set viewport bounds that are not exact matches
    element.viewportMinX = 15;
    element.viewportMaxX = 50;
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const mapping = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());

    // With continuous virtual indices:
    // 15 -> idx 0.5
    // 50 -> idx 1.375
    // Range = 0.875
    // Point 20 (idx 1) is at (1 - 0.5) / 0.875 = 57.14% of width.

    expect(mapping.mapX(20)).to.be.closeTo(591.4, 0.1);
  });

  it('maintains constant spacing during panning in even spacing mode', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
          { commit_number: 30, val: 3, createdat: 2500 },
          { commit_number: 100, val: 4, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const mapping1 = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());
    const spacing1 = mapping1.mapX(30) - mapping1.mapX(20);

    (element as any)['_dragCtx'] = {
      isDragging: true,
      dragStartX: 500,
      dragStartY: 200,
      isCtrl: false,
      isShift: false,
    };

    const moveEvent = new PointerEvent('pointermove', { clientX: 444, clientY: 200 });
    (element as any)['_handlePointerMove'](moveEvent);
    await element.updateComplete;

    const mapping2 = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());
    const spacing2 = mapping2.mapX(30) - mapping2.mapX(20);

    expect(spacing2).to.equal(spacing1);
  });

  it('aligns multiple traces correctly in even spacing mode', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'traceA',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
        ],
      },
      {
        id: 'traceB',
        color: '#000',
        rows: [
          { commit_number: 10, val: 3, createdat: 1000 },
          { commit_number: 30, val: 4, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const mapping = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());

    const x10_A = mapping.mapX(10);
    const x20_A = mapping.mapX(20);
    const x10_B = mapping.mapX(10);
    const x30_B = mapping.mapX(30);

    expect(x10_A).to.equal(x10_B);

    const diff1 = x20_A - x10_A;
    const diff2 = x30_B - x20_A;

    expect(Math.abs(diff1 - diff2)).to.be.below(1);
  });

  it('does not expand points when panning beyond edges in even spacing mode', async () => {
    element.evenXAxisSpacing = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 10, val: 1, createdat: 1000 },
          { commit_number: 20, val: 2, createdat: 2000 },
          { commit_number: 30, val: 3, createdat: 2500 },
          { commit_number: 100, val: 4, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    element.viewportMinX = 20;
    element.viewportMaxX = 100;
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const mapping1 = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());
    const spacing1 = mapping1.mapX(30) - mapping1.mapX(20);

    (element as any)['_dragCtx'] = {
      isDragging: true,
      dragStartX: 500,
      dragStartY: 200,
      isCtrl: false,
      isShift: false,
    };

    const moveEvent = new PointerEvent('pointermove', { clientX: 250, clientY: 200 });
    (element as any)['_handlePointerMove'](moveEvent);
    await element.updateComplete;

    const mapping2 = (element as any)['_getChartBoundsAndMapping'](canvas.getBoundingClientRect());
    const spacing2 = mapping2.mapX(30) - mapping2.mapX(20);

    expect(spacing2).to.equal(spacing1);
  });

  it('finds closest point correctly in dateMode', async () => {
    element.dateMode = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 1, val: 10.0, createdat: 1000 },
          { commit_number: 2, val: 20.0, createdat: 2000 },
          { commit_number: 3, val: 30.0, createdat: 3000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const oldGetMapping = (element as any)['_getChartBoundsAndMapping'];
    (element as any)['_getChartBoundsAndMapping'] = () => ({
      minX: 1000,
      maxX: 3000,
      padding: { left: 0, top: 0, right: 0, bottom: 0 },
      graphWidth: 1000,
      graphHeight: 400,
      mapX: (val: number) => (val - 1000) / 2,
      unmapX: (px: number) => 1000 + px * 2,
      mapY: (val: number) => 400 - val * 10,
      unmapY: (py: number) => (400 - py) / 10,
    });

    try {
      const moveEvent = new PointerEvent('pointermove', { clientX: 500, clientY: 200 });
      (element as any)['_handlePointerMove'](moveEvent);

      expect((element as any)['_hoveredPoint']).to.not.be.null;
      expect((element as any)['_hoveredPoint']!.row.commit_number).to.equal(2);
    } finally {
      (element as any)['_getChartBoundsAndMapping'] = oldGetMapping;
    }
  });

  it('finds closest point correctly in dateMode with non-monotonic data', async () => {
    element.dateMode = true;
    element.series = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 1, val: 10.0, createdat: 1000 },
          { commit_number: 2, val: 20.0, createdat: 3000 },
          { commit_number: 3, val: 30.0, createdat: 2000 },
        ],
      },
    ];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 400 }) as DOMRect;

    const oldGetMapping = (element as any)['_getChartBoundsAndMapping'];
    (element as any)['_getChartBoundsAndMapping'] = () => ({
      minX: 1000,
      maxX: 3000,
      padding: { left: 0, top: 0, right: 0, bottom: 0 },
      graphWidth: 1000,
      graphHeight: 400,
      mapX: (val: number) => (val - 1000) * 0.5,
      unmapX: (px: number) => 1000 + px * 2,
      mapY: (val: number) => 400 - val * 10,
      unmapY: (py: number) => (400 - py) / 10,
    });

    try {
      const moveEvent = new PointerEvent('pointermove', { clientX: 0, clientY: 300 });
      (element as any)['_handlePointerMove'](moveEvent);

      expect((element as any)['_hoveredPoint']).to.not.be.null;
      expect((element as any)['_hoveredPoint']!.row.commit_number).to.equal(1);
    } finally {
      (element as any)['_getChartBoundsAndMapping'] = oldGetMapping;
    }
  });

  it('draws extra circles for highlighted anomalies', async () => {
    element.regressions = {
      test: {
        100: {
          id: 'anomaly-123',
          commit_number: 100,
        } as any,
      },
    };
    element.highlightAnomalies = ['anomaly-123'];
    await element.updateComplete;

    const canvas = element.shadowRoot!.querySelector('#chart-canvas') as HTMLCanvasElement;
    const ctx = canvas.getContext('2d')!;
    const oldArc = ctx.arc;
    const arcRadii: number[] = [];
    ctx.arc = function (
      x: number,
      y: number,
      radius: number,
      startAngle: number,
      endAngle: number,
      counterclockwise?: boolean
    ) {
      arcRadii.push(radius);
      oldArc.call(this, x, y, radius, startAngle, endAngle, counterclockwise);
    };

    try {
      (element as any)['_processedSeries'] = [
        {
          id: 'test',
          color: '#fff',
          rows: [{ commit_number: 100, val: 10, createdat: 1000 }],
        },
      ];
      (element as any)['_drawBackground']();

      // Should have drawn concentric circles (radii 11 and 9) in addition to normal dot (radius 5 or 1.5)
      expect(arcRadii).to.include(11);
      expect(arcRadii).to.include(9);
    } finally {
      ctx.arc = oldArc;
    }
  });

  it('keeps hovered point on pointer leave if pinned', async () => {
    const hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    (element as any)['_hoveredPoint'] = hoveredPoint;
    element.globalPinnedX = 100;
    (element as any)['_mousePos'] = { x: 100, y: 100 };
    await element.updateComplete;

    (element as any)['_handlePointerLeave']();
    await element.updateComplete;

    expect((element as any)['_hoveredPoint']).to.not.be.null;
    expect((element as any)['_hoveredPoint']).to.equal(hoveredPoint);
    expect((element as any)['_mousePos']).to.be.null;

    element.globalPinnedX = null;
    (element as any)['_mousePos'] = { x: 100, y: 100 };
    await element.updateComplete;

    (element as any)['_handlePointerLeave']();
    await element.updateComplete;

    expect((element as any)['_hoveredPoint']).to.be.null;
    expect((element as any)['_mousePos']).to.be.null;
  });
});

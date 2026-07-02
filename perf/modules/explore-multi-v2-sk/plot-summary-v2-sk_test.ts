import './plot-summary-v2-sk';
import { PlotSummaryV2Sk } from './plot-summary-v2-sk';
import { expect } from 'chai';
import { TraceSeries } from './trace-types';

describe('plot-summary-v2-sk', () => {
  let element: PlotSummaryV2Sk;

  beforeEach(async () => {
    element = document.createElement('plot-summary-v2-sk') as PlotSummaryV2Sk;
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('instantiates the component with skeletal markup', () => {
    const container = element.shadowRoot!.querySelector('.summary-container');
    expect(container).to.not.be.null;
    const canvas = element.shadowRoot!.querySelector('canvas');
    expect(canvas).to.not.be.null;
    const box = element.shadowRoot!.querySelector('h-resizable-box-sk');
    expect(box).to.not.be.null;
  });

  it('does not throw when drawing empty series', () => {
    element.series = [];
    expect(() => element.drawSummary()).to.not.throw();
  });

  it('decimates trace rows correctly using min-max bucket when points exceed limit', () => {
    const rows = [];
    for (let i = 0; i < 1000; i++) {
      rows.push({ commit_number: i, val: i % 2 === 0 ? 10 : 100, createdat: i * 1000 });
    }

    const decimated = (element as any).decimate(rows);
    expect(decimated.length).to.be.below(rows.length);
    expect(decimated.length).to.be.at.most(1000);
  });

  it('draws trace lines on the canvas for date domain', async () => {
    const canvas = element.shadowRoot!.querySelector('canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 45 }) as DOMRect;

    const series: TraceSeries[] = [
      {
        id: 'trace1',
        color: '#ff0000',
        rows: [
          { commit_number: 10, val: 50, createdat: 1000 },
          { commit_number: 20, val: 150, createdat: 2000 },
        ],
      },
    ];

    element.domain = 'date';
    element.series = series;
    await element.updateComplete;

    const ctx = canvas.getContext('2d')!;
    const spyLines: { x: number; y: number; type: string }[] = [];
    const oldMoveTo = ctx.moveTo;
    const oldLineTo = ctx.lineTo;

    ctx.moveTo = function (x: number, y: number) {
      spyLines.push({ x, y, type: 'moveTo' });
      oldMoveTo.call(ctx, x, y);
    };
    ctx.lineTo = function (x: number, y: number) {
      spyLines.push({ x, y, type: 'lineTo' });
      oldLineTo.call(ctx, x, y);
    };

    try {
      element.drawSummary();
      expect(spyLines.length).to.equal(2);
      expect(spyLines[0].type).to.equal('moveTo');
      expect(spyLines[0].x).to.equal(0);
      expect(spyLines[1].type).to.equal('lineTo');
      expect(spyLines[1].x).to.equal(200);
    } finally {
      ctx.moveTo = oldMoveTo;
      ctx.lineTo = oldLineTo;
    }
  });

  it('aligns plot points when evenXAxisSpacing is enabled', async () => {
    const canvas = element.shadowRoot!.querySelector('canvas') as HTMLCanvasElement;
    canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 45 }) as DOMRect;

    const series: TraceSeries[] = [
      {
        id: 'trace1',
        color: '#ff0000',
        rows: [
          { commit_number: 10, val: 50, createdat: 1000 },
          { commit_number: 20, val: 100, createdat: 2000 },
          { commit_number: 100, val: 50, createdat: 3000 },
        ],
      },
    ];

    element.evenXAxisSpacing = true;
    element.domain = 'commit';
    element.series = series;
    await element.updateComplete;

    const ctx = canvas.getContext('2d')!;
    const spyPoints: number[] = [];
    const oldMoveTo = ctx.moveTo;
    const oldLineTo = ctx.lineTo;

    ctx.moveTo = function (x: number, y: number) {
      spyPoints.push(x);
      oldMoveTo.call(ctx, x, y);
    };
    ctx.lineTo = function (x: number, y: number) {
      spyPoints.push(x);
      oldLineTo.call(ctx, x, y);
    };

    try {
      element.drawSummary();
      expect(spyPoints.length).to.equal(3);
      const spacing1 = spyPoints[1] - spyPoints[0];
      const spacing2 = spyPoints[2] - spyPoints[1];
      expect(Math.abs(spacing1 - spacing2)).to.be.below(1);
    } finally {
      ctx.moveTo = oldMoveTo;
      ctx.lineTo = oldLineTo;
    }
  });

  it('converts domain values to pixel coords correctly when evenXAxisSpacing is enabled', async () => {
    element.series = [
      {
        id: 'trace1',
        color: '#ff0000',
        rows: [
          { commit_number: 10, val: 50, createdat: 1000 },
          { commit_number: 20, val: 100, createdat: 2000 },
          { commit_number: 100, val: 50, createdat: 3000 },
        ],
      },
    ];
    element.evenXAxisSpacing = true;
    element.domain = 'commit';
    await element.updateComplete;

    // Width = 200px. Unique commits: 10 (idx 0), 20 (idx 1), 100 (idx 2).
    // viewportMinX=20 (idx 1, 100px), viewportMaxX=100 (idx 2, 200px)
    const coords = (element as any).convertToCoordsRange(20, 100, 200);
    expect(coords).to.not.be.null;
    expect(coords!.begin).to.equal(100);
    expect(coords!.end).to.equal(200);
  });

  it('converts pixel coords to domain values correctly when evenXAxisSpacing is enabled', async () => {
    element.series = [
      {
        id: 'trace1',
        color: '#ff0000',
        rows: [
          { commit_number: 10, val: 50, createdat: 1000 },
          { commit_number: 20, val: 100, createdat: 2000 },
          { commit_number: 100, val: 50, createdat: 3000 },
        ],
      },
    ];
    element.evenXAxisSpacing = true;
    element.domain = 'commit';
    await element.updateComplete;

    // Width = 200px. 0px -> idx 0 (commit 10), 100px -> idx 1 (commit 20), 200px -> idx 2 (commit 100).
    const values = (element as any).convertToValueRange(100, 200, 200);
    expect(values).to.not.be.null;
    expect(values!.begin).to.equal(20);
    expect(values!.end).to.equal(100);
  });

  it('handles out-of-bounds coordinate and value conversions when evenXAxisSpacing is enabled', async () => {
    element.series = [
      {
        id: 'trace1',
        color: '#ff0000',
        rows: [
          { commit_number: 10, val: 50, createdat: 1000 },
          { commit_number: 20, val: 100, createdat: 2000 },
        ],
      },
    ];
    element.evenXAxisSpacing = true;
    element.domain = 'commit';
    await element.updateComplete;

    // arr = [10, 20], step = 10. width = 100px (0px=10, 100px=20).
    // -50px should map to virtIdx = -0.5 -> commit 5.
    const values = (element as any).convertToValueRange(-50, 150, 100);
    expect(values).to.not.be.null;
    expect(values!.begin).to.equal(5);
    expect(values!.end).to.equal(25);

    // Commit 0 (virtIdx = -1.0) should map to -100px. Commit 30 (virtIdx = 2.0) should map to 200px.
    const coords = (element as any).convertToCoordsRange(0, 30, 100);
    expect(coords).to.not.be.null;
    expect(coords!.begin).to.equal(-100);
    expect(coords!.end).to.equal(200);
  });
});

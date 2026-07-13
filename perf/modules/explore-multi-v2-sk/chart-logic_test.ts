import { expect } from 'chai';
import {
  paginateTraces,
  computeChartDimensions,
  calculateLoadedBounds,
  computeLeftPadding,
  calculateSharedBounds,
  computeSplitGroups,
} from './chart-logic';

describe('calculateLoadedBounds', () => {
  it('calculates min and max commit numbers for each trace', () => {
    const series = [
      {
        id: 'trace1',
        rows: [{ commit_number: 10 }, { commit_number: 30 }, { commit_number: 20 }],
      },
      {
        id: 'trace2',
        rows: [{ commit_number: 5 }, { commit_number: 15 }],
      },
    ];
    const bounds = calculateLoadedBounds(series as any);
    expect(bounds['trace1']).to.deep.equal({ min: 10, max: 30 });
    expect(bounds['trace2']).to.deep.equal({ min: 5, max: 15 });
  });

  it('handles empty rows', () => {
    const series = [{ id: 'trace1', rows: [] }];
    const bounds = calculateLoadedBounds(series as any);
    expect(bounds['trace1']).to.be.undefined;
  });
});

describe('chart-logic paginateTraces', () => {
  it('should paginate traces correctly', () => {
    const groups = [
      [{ id: 'a' }, { id: 'b' }],
      [{ id: 'c' }, { id: 'd' }, { id: 'e' }],
    ];
    // We expect input to be an array of arrays of traces (series from groups)
    const pages = paginateTraces(groups as any, 2);
    expect(pages.length).to.equal(3);
    expect(pages[0]).to.deep.equal([{ id: 'a' }, { id: 'b' }]);
    expect(pages[1]).to.deep.equal([{ id: 'c' }, { id: 'd' }]);
    expect(pages[2]).to.deep.equal([{ id: 'e' }]);
  });
});

describe('chart-logic computeChartDimensions', () => {
  it('should return empty array for empty series', () => {
    expect(computeChartDimensions([])).to.deep.equal([]);
  });

  it('should identify differing keys', () => {
    const series = [
      { id: 'benchmark=A,bot=X,unit=ms' },
      { id: 'benchmark=A,bot=Y,unit=ms' },
      { id: 'benchmark=B,bot=X,unit=ms' },
    ];
    expect(computeChartDimensions(series)).to.deep.equal(['benchmark', 'bot']);
  });

  it('should ignore hidden params', () => {
    const series = [{ id: 'benchmark=A,unit=ms' }, { id: 'benchmark=A,unit=s' }];
    expect(computeChartDimensions(series)).to.deep.equal([]);
  });

  it('should handle missing keys', () => {
    const series = [{ id: 'benchmark=A,bot=X' }, { id: 'benchmark=A' }];
    expect(computeChartDimensions(series)).to.deep.equal(['bot']);
  });
});

describe('computeLeftPadding', () => {
  it('should return larger padding for large numbers', () => {
    const padding = computeLeftPadding(8895461, 2161590);
    expect(padding).to.be.greaterThan(60);
  });
});

describe('calculateSharedBounds', () => {
  it('should calculate bounds across multiple series', () => {
    const series = [
      { id: 't1', source: 'chrome', rows: [{ commit_number: 10 }, { commit_number: 30 }] },
      { id: 't2', source: 'chrome', rows: [{ commit_number: 20 }, { commit_number: 40 }] },
    ];
    const bounds = calculateSharedBounds(series, null);
    expect(bounds).to.deep.equal({
      chrome: { min: 10, max: 40 },
    });
  });
});

describe('computeSplitGroups', () => {
  it('should not split by unit by default if not in splitKeys', () => {
    const series = [
      { id: 'benchmark=A,unit=ms', rows: [] },
      { id: 'benchmark=A,unit=s', rows: [] },
    ];
    const groups = computeSplitGroups(series as any, new Set());
    expect(groups.length).to.equal(1);
    expect(groups[0].title).to.equal('benchmark=A');
  });

  it('should split every trace into its own graph when splitAll is true', () => {
    const series = [
      { id: 'benchmark=A,unit=ms', rows: [] },
      { id: 'benchmark=A,unit=s', rows: [] },
    ];
    const groups = computeSplitGroups(series as any, new Set(), true);
    expect(groups.length).to.equal(2);
    expect(groups[0].id).to.equal('benchmark=A,unit=ms');
    expect(groups[1].id).to.equal('benchmark=A,unit=s');
  });
});

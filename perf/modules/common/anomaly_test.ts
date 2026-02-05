import { assert } from 'chai';
import { formatPercentage } from './anomaly';

describe('formatPercentage', () => {
  it('returns percentage with a positive sign', () => {
    const formattedPerc = formatPercentage(72);
    assert.equal(formattedPerc, `+72`);
  });

  it('returns percentage with a negative sign', () => {
    const formattedPerc = formatPercentage(-72);
    assert.equal(formattedPerc, `-72`);
  });
});

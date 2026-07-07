import { expect } from 'chai';
import { fuzzyScore, scoreParam, scoreParamAny, globRegexCache } from './fuzzy';

describe('fuzzy matching for Android benchmark class names (b/531657261)', () => {
  it('should score class name segment matches higher than package-only matches', () => {
    const classNameMatch = fuzzyScore('androidx.compose.runtime.ComposeBenchmark', 'Compose');
    const packageOnlyMatch = fuzzyScore('androidx.compose.runtime.OtherBenchmark', 'Compose');

    expect(classNameMatch).to.be.greaterThan(packageOnlyMatch);
  });

  it('should return exact match score for matching full class name segment', () => {
    const score = fuzzyScore('androidx.compose.runtime.ComposeBenchmark', 'ComposeBenchmark');
    expect(score).to.be.greaterThanOrEqual(100000);
  });

  it('should match LazyListScroll to LazyListScrollingBenchmark', () => {
    const score = fuzzyScore(
      'androidx.compose.foundation.benchmark.LazyListScrollingBenchmark',
      'LazyListScroll'
    );
    expect(score).to.be.greaterThan(500);
  });

  it('should score test_class=ComposeBenchmark with high priority', () => {
    const param = { key: 'test_class', value: 'androidx.compose.runtime.ComposeBenchmark' };
    const score = scoreParam(param, 'test_class=ComposeBenchmark');

    expect(score).to.be.greaterThanOrEqual(100000);
  });

  it('should boost keys based on their position in includeParams', () => {
    const archParam = { key: 'arch', value: 'arm64' };
    const osParam = { key: 'os', value: 'android' };

    const includeParams1 = ['arch', 'os'];
    const archScore1 = scoreParamAny(archParam, 'arm', includeParams1);
    const osScore1 = scoreParamAny(osParam, 'and', includeParams1);
    expect(archScore1).to.be.greaterThan(osScore1);

    const includeParams2 = ['os', 'arch'];
    const archScore2 = scoreParamAny(archParam, 'arm', includeParams2);
    const osScore2 = scoreParamAny(osParam, 'and', includeParams2);
    expect(osScore2).to.be.greaterThan(archScore2);
  });

  it('evicts oldest cache entries when limit is reached (LRU)', () => {
    globRegexCache.clear();

    for (let i = 0; i < 1000; i++) {
      scoreParam({ key: 'a', value: 'b' }, `a=val${i}*`);
    }
    expect(globRegexCache.size).to.equal(1000);
    expect(globRegexCache.has('val0*')).to.be.true;

    scoreParam({ key: 'a', value: 'b' }, 'a=val0*');

    scoreParam({ key: 'a', value: 'b' }, 'a=val1000*');

    expect(globRegexCache.size).to.equal(1000);
    expect(globRegexCache.has('val0*')).to.be.true;
    expect(globRegexCache.has('val1*')).to.be.false;
    expect(globRegexCache.has('val1000*')).to.be.true;
  });
});

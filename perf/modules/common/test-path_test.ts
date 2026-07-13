import { assert } from 'chai';
import { parseTestPath } from './test-path';

describe('parseTestPath', () => {
  it('parses a standard test path with chart and story', () => {
    const parsed = parseTestPath(
      'ChromiumPerf/linux-perf/blink_perf.rendering/draw-canvas/draw-canvas-story'
    );
    assert.deepEqual(parsed, {
      bot: 'ChromiumPerf',
      configuration: 'linux-perf',
      benchmark: 'blink_perf.rendering',
      chart: 'draw-canvas',
      statistic: '',
      story: 'draw-canvas-story',
    });
  });

  it('parses a test path with statistic in chart name', () => {
    const parsed = parseTestPath('ChromiumPerf/mac-perf/rendering.desktop/score:avg/story_1');
    assert.deepEqual(parsed, {
      bot: 'ChromiumPerf',
      configuration: 'mac-perf',
      benchmark: 'rendering.desktop',
      chart: 'score',
      statistic: 'avg',
      story: 'story_1',
    });
  });

  it('handles empty test path gracefully', () => {
    const parsed = parseTestPath('');
    assert.deepEqual(parsed, {
      bot: '',
      configuration: '',
      benchmark: '',
      chart: '',
      statistic: '',
      story: '',
    });
  });
});

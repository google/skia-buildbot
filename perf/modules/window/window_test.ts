import { getBuildTag } from './window';
import { assert } from 'chai';

describe('window', () => {
  it('image git tag', () => {
    assert.deepEqual(
      getBuildTag('gcr.io/skia-public/perfserver@tag:git-d1fe6fd902abefd87a4e30f5317cfaf71246cfd4'),
      { type: 'git', tag: 'd1fe6fd' }
    );
  });

  it('incorrect image tag', () => {
    assert.deepEqual(getBuildTag(''), { type: 'invalid', tag: null });
    assert.deepEqual(getBuildTag('@'), { type: 'invalid', tag: null });
    assert.deepEqual(getBuildTag('gcr@ta-'), { type: 'invalid', tag: null });
    assert.deepEqual(getBuildTag('@@gcr@tag'), { type: 'invalid', tag: null });
    assert.deepEqual(getBuildTag('@tag@tag'), { type: 'invalid', tag: null });
  });

  it('louhi build tag', () => {
    assert.deepEqual(
      getBuildTag('gcr.io/skia-public/perfserver@tag:2024-10-08T05_08_05Z-louhi-b5d0f9c-clean'),
      { type: 'louhi', tag: 'b5d0f9c' }
    );
  });

  it('arbitrary image tag', () => {
    assert.deepEqual(getBuildTag('gcr.io/skia-public/perfserver@tag:latest'), {
      type: 'tag',
      tag: 'latest',
    });
    assert.deepEqual(getBuildTag('gcr.io/skia-public/perfserver@tag:v1.0'), {
      type: 'tag',
      tag: 'v1.0',
    });
  });
});

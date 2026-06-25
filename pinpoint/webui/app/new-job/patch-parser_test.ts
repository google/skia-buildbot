import { parsePatch } from './patch-parser';
import { assert } from 'chai';

describe('patch-parser', () => {
  it('should throw on empty input', () => {
    assert.throws(() => parsePatch(''), 'Could not parse an empty string');
    assert.throws(() => parsePatch('   '), 'Could not parse an empty string');
  });

  it('should parse change ID', () => {
    assert.deepEqual(parsePatch('123456'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
    });
  });

  it('should parse change ID with patchset', () => {
    assert.deepEqual(parsePatch('123456/3'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
      patchset: 3,
    });
  });

  it('should throw when too many numbers', () => {
    assert.throws(() => parsePatch('123456/3/1'));
  });

  it('should throw when patchset is 0', () => {
    assert.throws(() => parsePatch('123456/0'));
  });

  it('should parse crrev.com URL', () => {
    assert.deepEqual(parsePatch('crrev.com/c/123456'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
    });
    assert.deepEqual(parsePatch('https://crrev.com/123456/2'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
      patchset: 2,
    });
    assert.deepEqual(parsePatch('crrev/123456'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
    });
    assert.deepEqual(parsePatch('crrev/c/123456/3'), {
      host: 'https://chromium-review.googlesource.com',
      change: 123456,
      patchset: 3,
    });
  });

  it('should parse full Gerrit URL with project', () => {
    assert.deepEqual(
      parsePatch('https://chromium-review.git.corp.google.com/c/chromium/src/+/123456/3'),
      {
        host: 'https://chromium-review.git.corp.google.com',
        change: 123456,
        patchset: 3,
      }
    );
  });

  it('should parse full Gerrit URL without project', () => {
    assert.deepEqual(parsePatch('https://chromium-review.git.corp.google.com/+/123456'), {
      host: 'https://chromium-review.git.corp.google.com',
      change: 123456,
    });
  });

  it('should parse full Gerrit URL with c path', () => {
    assert.deepEqual(parsePatch('https://chromium-review.git.corp.google.com/c/123456/2'), {
      host: 'https://chromium-review.git.corp.google.com',
      change: 123456,
      patchset: 2,
    });
  });

  it('should parse URL path fallback', () => {
    assert.deepEqual(parsePatch('https://chromium-review.git.corp.google.com/123456'), {
      host: 'https://chromium-review.git.corp.google.com',
      change: 123456,
    });
  });

  it('should throw for invalid inputs', () => {
    assert.throws(() => parsePatch('not-a-patch'));
    assert.throws(() => parsePatch('https://google.com'));
    assert.throws(() => parsePatch('https://chromium-review.com/123456'));
    assert.throws(
      () => parsePatch('http://chromium-review.git.corp.google.com/123456'),
      'HTTP protocol is not allowed. Please use HTTPS.'
    );
  });
});

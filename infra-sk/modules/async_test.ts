import { asyncFind, asyncForEach, asyncMap } from './async';
import { expect } from 'chai';

describe('async utilities', () => {
  describe('asyncFind', () => {
    it('handles empty inputs', async () => {
      expect(await asyncFind(undefined, async () => true)).to.be.undefined;
      expect(await asyncFind([], async () => true)).to.be.undefined;
      expect(await asyncFind(wrapInPromise([]), async () => true)).to.be.undefined;
    });

    const haystack = ["alpha", "beta", "gamma", "delta"];

    it('finds it', async() => {
      const finderFn = async (needle: string) => (await wrapInPromise(needle)).startsWith('g');
      expect(await asyncFind(haystack, finderFn)).to.equal("gamma");
      expect(await asyncFind(wrapInPromise(haystack), finderFn)).to.equal("gamma");
    })

    it('does not find it', async () => {
      const finderFn = async (needle: string) => await wrapInPromise(false);
      expect(await asyncFind(haystack, finderFn)).to.be.undefined;
      expect(await asyncFind(wrapInPromise(haystack), finderFn)).to.be.undefined;
    });
  });

  describe('asyncMap', () => {
    it('maps empty inputs', async () => {
      expect(await asyncMap(undefined, async () => undefined)).to.deep.equal([]);
      expect(await asyncMap([], async () => undefined)).to.deep.equal([]);
      expect(await asyncMap(wrapInPromise([]), async () => undefined)).to.deep.equal([]);
    });

    it('maps non-empty inputs', async () => {
      const input = ["hello", "world"];
      const expectedOutput = ["HELLO", "WORLD"];
      const mapper = async (s: string) => await wrapInPromise(s.toUpperCase());
      expect(await asyncMap(input, mapper)).to.deep.equal(expectedOutput);
      expect(await asyncMap(wrapInPromise(input), mapper)).to.deep.equal(expectedOutput);
    })
  });

  describe('asyncForEach', () => {
    it('does not iterate on empty inputs', async () => {
      let numCalls = 0;
      const callbackFn = async (s: string) => {
        await wrapInPromise(undefined);
        numCalls++;
      }

      await asyncForEach(undefined, callbackFn);
      expect(numCalls).to.equal(0);
      await asyncForEach([], callbackFn);
      expect(numCalls).to.equal(0);
      await asyncForEach(wrapInPromise([]), callbackFn);
      expect(numCalls).to.equal(0);
    });

    it('iterates on non-empty inputs', async () => {
      const input = ['alpha', 'beta', 'gamma'];
      let callbacks: string[] = [];
      const callbackFn = async (s: string) => {
        await wrapInPromise(undefined);
        callbacks.push(s);
      }

      await asyncForEach(input, callbackFn);
      expect(callbacks).to.deep.equal(input);
      callbacks = [];
      await asyncForEach(wrapInPromise(input), callbackFn);
      expect(callbacks).to.deep.equal(input);
    })
  });
});

const wrapInPromise = <T>(t: T) => new Promise<T>((resolve) => resolve(t));

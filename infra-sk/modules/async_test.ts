import { asyncFilter, asyncFind, asyncForEach, asyncMap } from './async';
import { expect } from 'chai';

describe('async utilities', () => {
  describe('asyncFind', () => {
    it('handles empty inputs', async () => {
      const finderFn = async (_: any) => {
        await simulateAsyncOp();
        return true;
      }
      expect(await asyncFind(undefined, finderFn)).to.be.undefined;
      expect(await asyncFind([], finderFn)).to.be.undefined;
      expect(await asyncFind(wrapInPromise([]), finderFn)).to.be.undefined;
    });

    const haystack = ['alpha', 'beta', 'gamma', 'delta'];

    it('finds it', async() => {
      const finderFn = async (needle: string) => {
        await simulateAsyncOp();
        return needle.startsWith('g');
      }
      const expectedOutput = 'gamma';
      expect(await asyncFind(haystack, finderFn)).to.equal(expectedOutput);
      expect(await asyncFind(wrapInPromise(haystack), finderFn)).to.equal(expectedOutput);
    })

    it('does not find it', async () => {
      const finderFn = async (_: string) => {
        await simulateAsyncOp();
        return false;
      }
      expect(await asyncFind(haystack, finderFn)).to.be.undefined;
      expect(await asyncFind(wrapInPromise(haystack), finderFn)).to.be.undefined;
    });
  });

  describe('asyncFilter', () => {
    it('handles empty inputs', async () => {
      const filterFn = async (_: any) => {
        await simulateAsyncOp();
        return true;
      }
      expect(await asyncFilter(undefined, filterFn)).to.be.empty;
      expect(await asyncFilter([], filterFn)).to.be.empty;
      expect(await asyncFilter(wrapInPromise([]), filterFn)).to.be.empty;
    });

    it('filters the input', async () => {
      const filterFn = async (item: string) => {
        await simulateAsyncOp();
        return item.startsWith('a') || item.startsWith('g');
      };
      const input = ['alpha', 'beta', 'gamma', 'delta'];
      const expectedOutput = ['alpha', 'gamma'];
      expect(await asyncFilter(input, filterFn)).to.deep.equal(expectedOutput);
      expect(await asyncFilter(wrapInPromise(input), filterFn)).to.deep.equal(expectedOutput);
    })
  });

  describe('asyncMap', () => {
    it('maps empty inputs', async () => {
      const mapperFn = async (_: any) => {
        await simulateAsyncOp();
        return 'hello';
      }
      expect(await asyncMap(undefined, mapperFn)).to.be.empty;
      expect(await asyncMap([], mapperFn)).to.be.empty;
      expect(await asyncMap(wrapInPromise([]), mapperFn)).to.be.empty;
    });

    it('maps non-empty inputs', async () => {
      const mapperFn = async (s: string) => {
        await simulateAsyncOp();
        return s.toUpperCase();
      }
      const input = ["hello", "world"];
      const expectedOutput = ["HELLO", "WORLD"];
      expect(await asyncMap(input, mapperFn)).to.deep.equal(expectedOutput);
      expect(await asyncMap(wrapInPromise(input), mapperFn)).to.deep.equal(expectedOutput);
    })
  });

  describe('asyncForEach', () => {
    it('does not iterate on empty inputs', async () => {
      let numCalls = 0;
      const callbackFn = async (_: string) => {
        await simulateAsyncOp();
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
      let callbacks: string[] = [];
      const callbackFn = async (s: string) => {
        await simulateAsyncOp();
        callbacks.push(s);
      }
      const input = ['alpha', 'beta', 'gamma'];

      await asyncForEach(input, callbackFn);
      expect(callbacks).to.deep.equal(input);

      callbacks = [];
      await asyncForEach(wrapInPromise(input), callbackFn);
      expect(callbacks).to.deep.equal(input);
    })
  });
});

async function simulateAsyncOp(): Promise<void> {}

function wrapInPromise<T>(value: T): Promise<T> {
  return new Promise((resolve) => resolve(value));
}

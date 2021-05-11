import { asyncFilter, asyncFind, asyncForEach, asyncMap } from './async';
import { expect } from 'chai';

describe('async utilities', () => {
  describe('asyncFind', () => {
    it('handles empty inputs', async () => {
      const visitedItems: number[] = [];
      const finderFn = async (item: any, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        return true;
      }

      // Raw array.
      expect(await asyncFind([], finderFn)).to.be.null;
      expect(visitedItems).to.be.empty;

      // Array wrapped in promise.
      expect(await asyncFind(wrapInPromise([]), finderFn)).to.be.null;
      expect(visitedItems).to.be.empty;
    });

    it('finds it', async() => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const finderFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
        return item.startsWith('g');
      }
      const input = ['alpha', 'beta', 'gamma', 'delta'];

      // Raw array.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFind(input, finderFn)).to.equal('gamma');
      expect(visitedItems).to.deep.equal(['alpha', 'beta', 'gamma']);
      expect(visitedIndices).to.deep.equal([0, 1, 2]);

      // Array wrapped in promise.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFind(wrapInPromise(input), finderFn)).to.equal('gamma');
      expect(visitedItems).to.deep.equal(['alpha', 'beta', 'gamma']);
      expect(visitedIndices).to.deep.equal([0, 1, 2]);
    });

    it('does not find it', async () => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const finderFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
        return false; // Never finds it.
      }
      const input = ['alpha', 'beta', 'gamma', 'delta'];

      // Raw array.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFind(input, finderFn)).to.be.null;
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2, 3]);

      // Array wrapped in promise.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFind(wrapInPromise(input), finderFn)).to.be.null;
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2, 3]);
    });
  });

  describe('asyncFilter', () => {
    it('handles empty inputs', async () => {
      const visitedIndices: number[] = [];
      const filterFn = async (item: any, index: number) => {
        await simulateAsyncOp();
        visitedIndices.push(index);
        return true;
      }

      // Raw array.
      expect(await asyncFilter([], filterFn)).to.be.empty;
      expect(visitedIndices).to.be.empty;

      // Array wrapped in promise.
      expect(await asyncFilter(wrapInPromise([]), filterFn)).to.be.empty;
      expect(visitedIndices).to.be.empty;
    });

    it('filters the input', async () => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const filterFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
        return item.startsWith('a') || item.startsWith('g');
      };
      const input = ['alpha', 'beta', 'gamma', 'delta'];

      // Raw array.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFilter(input, filterFn)).to.deep.equal(['alpha', 'gamma']);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2, 3]);

      // Array wrapped in promise.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncFilter(wrapInPromise(input), filterFn)).to.deep.equal(['alpha', 'gamma']);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2, 3]);
    })
  });

  describe('asyncMap', () => {
    it('maps empty inputs', async () => {
      const visitedIndices: number[] = [];
      const mapperFn = async (item: any, index: number) => {
        await simulateAsyncOp();
        visitedIndices.push(index);
        return 'hello';
      }

      // Raw array.
      expect(await asyncMap([], mapperFn)).to.be.empty;
      expect(visitedIndices).to.be.empty;

      // Array wrapped in promise.
      expect(await asyncMap(wrapInPromise([]), mapperFn)).to.be.empty;
      expect(visitedIndices).to.be.empty;
    });

    it('maps non-empty inputs', async () => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const mapperFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
        return item.toUpperCase();
      }
      const input = ["hello", "world"];

      // Raw array.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncMap(input, mapperFn)).to.deep.equal(['HELLO', 'WORLD']);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1]);

      // Array wrapped in promise.
      visitedItems = [];
      visitedIndices = [];
      expect(await asyncMap(wrapInPromise(input), mapperFn)).to.deep.equal(['HELLO', 'WORLD']);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1]);
    })
  });

  describe('asyncForEach', () => {
    it('does not iterate on empty inputs', async () => {
      const visitedIndices: number[] = [];
      const callbackFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedIndices.push(index);
      }

      // Raw array.
      await asyncForEach([], callbackFn);
      expect(visitedIndices).to.be.empty;

      // Array wrapped in promise.
      await asyncForEach(wrapInPromise([]), callbackFn);
      expect(visitedIndices).to.be.empty;
    });

    it('iterates on non-empty inputs', async () => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const callbackFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
      }
      const input = ['alpha', 'beta', 'gamma'];

      // Raw array.
      visitedItems = [];
      visitedIndices = [];
      await asyncForEach(input, callbackFn);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2]);

      // Array wrapped in promise.
      visitedItems = [];
      visitedIndices = [];
      await asyncForEach(wrapInPromise(input), callbackFn);
      expect(visitedItems).to.deep.equal(input);
      expect(visitedIndices).to.deep.equal([0, 1, 2]);
    })
  });
});

async function simulateAsyncOp(): Promise<void> {}

function wrapInPromise<T>(value: T): Promise<T> {
  return new Promise((resolve) => resolve(value));
}

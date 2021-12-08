import { expect } from 'chai';
import {
  asyncFilter, asyncFind, asyncForEach, asyncMap,
} from './async';

describe('async utilities', () => {
  describe('asyncFind', () => {
    it('handles empty inputs', async () => {
      const visitedItems: number[] = [];
      const finderFn = async (item: any) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        return true;
      };

      // Raw array.
      expect(await asyncFind([], finderFn)).to.be.null;
      expect(visitedItems).to.be.empty;

      // Array wrapped in promise.
      expect(await asyncFind(wrapInPromise([]), finderFn)).to.be.null;
      expect(visitedItems).to.be.empty;
    });

    it('finds it', async () => {
      let visitedItems: string[];
      let visitedIndices: number[];
      const finderFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedItems.push(item);
        visitedIndices.push(index);
        return item.startsWith('g');
      };
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
      };
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
      };

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
    });
  });

  describe('asyncMap', () => {
    it('maps empty inputs', async () => {
      const visitedIndices: number[] = [];
      const mapperFn = async (item: any, index: number) => {
        await simulateAsyncOp();
        visitedIndices.push(index);
        return 'hello';
      };

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
      };
      const input = ['hello', 'world'];

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
    });
  });

  describe('asyncForEach', () => {
    it('does not iterate on empty inputs', async () => {
      const visitedIndices: number[] = [];
      const callbackFn = async (item: string, index: number) => {
        await simulateAsyncOp();
        visitedIndices.push(index);
      };

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
      };
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
    });
  });
});

async function simulateAsyncOp(): Promise<void> {
  // Do nothing.
}

function wrapInPromise<T>(value: T): Promise<T> {
  return new Promise((resolve) => resolve(value));
}

describe('examples with and without async.ts', () => {
  // These examples illustrate situations involving multiple async operations where async.ts can
  // make the code simpler.

  const stateCapitalsMap = new Map<string, string>([
    ['Colorado', 'Denver'],
    ['Georgia', 'Atlanta'],
    ['Massachussets', 'Boston'],
    ['North Carolina', 'Raleigh'],
    ['Texas', 'Austin'],
  ]);

  const cityPopulationsMap = new Map<string, number>([
    ['Atlanta', 498_715],
    ['Austin', 950_807],
    ['Boston', 684_000],
    ['Denver', 705_576],
    ['Raleigh', 464_485],
  ]);

  // Simulates an RPC operation.
  const getStatesRPC = (): Promise<string[]> => wrapInPromise(Array.from(stateCapitalsMap.keys()));

  // Simulates an RPC operation.
  const getStateCapitalRPC = (state: string): Promise<string> => (
    wrapInPromise(stateCapitalsMap.get(state)!)
  );

  // Simulates an RPC operation.
  const getCityPopulationRPC = (city: string): Promise<number> => (
    wrapInPromise(cityPopulationsMap.get(city)!)
  );

  describe('without async.ts', () => {
    it('fetches state capitals', async () => {
      const states = await getStatesRPC();
      const stateCapitalPromises = states.map(getStateCapitalRPC);
      const stateCapitals = await Promise.all(stateCapitalPromises);

      expect(stateCapitals).to.have.length(5);
      expect(stateCapitals).to.have.members(['Atlanta', 'Austin', 'Boston', 'Denver', 'Raleigh']);
    });

    it('fetches state capitals with population > 500k', async () => {
      const states = await getStatesRPC();
      const stateCapitals = await Promise.all(states.map(getStateCapitalRPC));

      // We can't use Array.prototype.filter because async predicates return promises, which are
      // truthy, and thus nothing gets filtered out.
      const failedAttempt = stateCapitals.filter(
        async (city) => (await getCityPopulationRPC(city)) > 500_000,
      );
      expect(failedAttempt).to.have.length(5);

      const populousStateCapitals: string[] = [];
      await Promise.all(stateCapitals.map(async (city) => {
        const population = await getCityPopulationRPC(city);
        if (population > 500_000) populousStateCapitals.push(city);
      }));

      expect(populousStateCapitals).to.have.length(3);
      expect(populousStateCapitals).to.have.members(['Austin', 'Boston', 'Denver']);
    });
  });

  describe('with async.ts', () => {
    it('fetches state capitals', async () => {
      const stateCapitals = await asyncMap(getStatesRPC(), getStateCapitalRPC);

      expect(stateCapitals).to.have.length(5);
      expect(stateCapitals).to.have.members(['Atlanta', 'Austin', 'Boston', 'Denver', 'Raleigh']);
    });

    it('fetches state capitals with population > 500k', async () => {
      const stateCapitals = asyncMap(getStatesRPC(), getStateCapitalRPC);
      const populousStateCapitals = await asyncFilter(
        stateCapitals,
        async (city) => (await getCityPopulationRPC(city)) > 500_000,
      );

      expect(populousStateCapitals).to.have.length(3);
      expect(populousStateCapitals).to.have.members(['Austin', 'Boston', 'Denver']);
    });
  });
});

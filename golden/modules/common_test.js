import {
  humanReadableQuery,
} from './common';

describe('humanReadableQuery', () => {
  it('turns url encoded queries into human readable version', () => {
    expect(humanReadableQuery('alpha=beta&gamma=delta')).to.equal(
      'alpha=beta\ngamma=delta',
    );
    const inputWithSpaces = "mind%20the%20gap=tube&woody=There's%20a%20space%20in%20my%20boot";
    expect(humanReadableQuery(inputWithSpaces)).to.equal(
      "mind the gap=tube\nwoody=There's a space in my boot",
    );
  });
});

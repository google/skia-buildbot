import { expect } from 'chai';
import { DefaultMap } from './default-map';

describe('default-map', () => {
  it('initializes new items', () => {
    const d = new DefaultMap<string, number[]>(() => []);
    d.get('hello').push(2);
    d.get('hello').push(3);
    d.get('world').push(4);
    expect(d.get('hello')).to.deep.equal([2, 3]);
    expect(d.get('world')).to.deep.equal([4]);
    expect(d.size).to.equal(2);
  });
});

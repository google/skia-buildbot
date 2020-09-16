import { getDurationTillNextDay } from './am';

describe('am', () => {
  let clock;
  before(() => {
    clock = sinon.useFakeTimers();
  });
  after(() => {
    clock.restore();
  });

  it('tests getDurationTillNextDay', async () => {
    clock = sinon.useFakeTimers(new Date(1999, 0, 1, 9, 0, 0)); // Friday 9 AM
    const normalDate = new Date(1999, 0, 1, 9, 0, 0);
    // Friday to Monday.
    expect(getDurationTillNextDay(normalDate, 1, 9)).to.equal('3d');
    // Friday to Friday.
    expect(getDurationTillNextDay(normalDate, 5, 9)).to.equal('3d');
  });
});

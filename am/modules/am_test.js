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
    // Set fake clock to 1999 Jan 1st Friday 9 AM.
    clock = sinon.useFakeTimers(new Date(1999, 0, 1, 9, 0, 0));
    // Friday to Monday.
    expect(getDurationTillNextDay(1, 9)).to.equal('3d');
    // Friday to Thursday.
    expect(getDurationTillNextDay(4, 8)).to.equal('6d');
    // Friday to Friday same time.
    expect(getDurationTillNextDay(5, 9)).to.equal('1w');
    // Friday to Friday one hour later.
    expect(getDurationTillNextDay(5, 10)).to.equal('1w');
    // Friday to Friday one hour earlier.
    expect(getDurationTillNextDay(5, 8)).to.equal('1w');

    // Set fake clock to 1999 Dec 31st Friday 9 AM.
    clock = sinon.useFakeTimers(new Date(1999, 11, 31, 9, 0, 0));
    // Friday to Saturday.
    expect(getDurationTillNextDay(6, 9)).to.equal('1d');
    // Friday to Sunday.
    expect(getDurationTillNextDay(0, 9)).to.equal('2d');
    // Friday to Friday.
    expect(getDurationTillNextDay(5, 9)).to.equal('1w');
  });
});

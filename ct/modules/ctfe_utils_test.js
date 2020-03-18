import { getTimestamp, getFormattedTimestamp, getCtDbTimestamp } from './ctfe_utils';

describe('ctfe_utils', () => {
    // All dates are arbitrary.
  it('converts CT DB int date to JS Date', async () => {
    const date = getTimestamp(20200513095930);
    expect(date.getUTCFullYear()).to.equal(2020);
    // Month is 0 indexed in JS even though date(day) is not.
    expect(date.getUTCMonth()).to.equal(4);
    expect(date.getUTCDate()).to.equal(13);
    expect(date.getUTCHours()).to.equal(9);
    expect(date.getUTCMinutes()).to.equal(59);
    expect(date.getUTCSeconds()).to.equal(30);
  });

  it('converts CT DB int date to human readable string', async () => {
    const date_string = getFormattedTimestamp(20200513095930);
    expect(date_string).to.equal('5/13/2020, 5:59:30 AM');
  });

  it('converts a JS Date to a CT DB int date', async () => {
    const date = new Date('December 31, 1975, 23:15:30 GMT+11:00');
    const db_date = getCtDbTimestamp(date);
    expect(db_date).to.equal('19751231121530');
  });
});

import { getTimestamp, getFormattedTimestamp, getCtDbTimestamp } from './ctfe_utils';

describe('ctfe_utils', () => {
    // All dates are arbitrary.
  it('getTimestamp works', async () => {
    const date = getTimestamp(20200513095930);
    expect(date.getFullYear()).to.equal(2020);
    // Month is 0 indexed in JS even though date(day) is not.
    expect(date.getMonth()).to.equal(4);
    expect(date.getDate()).to.equal(13);
    expect(date.getHours()).to.equal(9);
    expect(date.getMinutes()).to.equal(59);
    expect(date.getSeconds()).to.equal(30);
  });

  it('getFormattedTimestamp works', async () => {
    const date_string = getFormattedTimestamp(20200513095930);
    expect(date_string).to.equal('5/13/2020, 9:59:30 AM');
  });

  it('getTimestamp works', async () => {
    const date = new Date('December 31, 1975, 23:15:30 GMT+11:00');
    const db_date = getCtDbTimestamp(date);
    expect(db_date).to.equal('19751231121530');
  });
});

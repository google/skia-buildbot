import { getTimestamp, getFormattedTimestamp, getCtDbTimestamp } from './ctfe_utils';

describe('ctfe_utils', () => {

  it('getTimestamp works', async () => {
    const date = getTimestamp(20200222200202);
    expect(date.getFullYear()).to.equal(2020);
    expect(date.getMonth()).to.equal(1);
    expect(date.getDate()).to.equal(22);
    expect(date.getHours()).to.equal(20);
    expect(date.getMinutes()).to.equal(2);
    expect(date.getSeconds()).to.equal(2);
  });

  it('getFormattedTimestamp works', async () => {
    const date_string = getFormattedTimestamp(20200222200202);
    expect(date_string).to.equal('2/22/2020, 8:02:02 PM');
  });

  it('getTimestamp works', async () => {
    const date = new Date('December 31, 1975, 23:15:30 GMT+11:00');
    const db_date = getCtDbTimestamp(date);
    expect(db_date).to.equal('19751231121530');
  });
});

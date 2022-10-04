import { assert } from 'chai';
import { pivot } from '../json/all';
import { validateAsPivotTable, validatePivotRequest } from './index';

describe('validatePivotRequest', () => {
  it('returns error message on null request', () => {
    assert.isNotEmpty(validatePivotRequest(null));
  });

  it('returns error message if group_by is null', () => {
    const req: pivot.Request = {
      group_by: null,
      operation: 'avg',
      summary: [],
    };
    assert.isNotEmpty(validatePivotRequest(req));
  });

  it('returns error message if group_by is empty', () => {
    const req: pivot.Request = {
      group_by: [],
      operation: 'avg',
      summary: [],
    };
    assert.isNotEmpty(validatePivotRequest(req));
  });

  it('returns no error message if entire request is valid', () => {
    const req: pivot.Request = {
      group_by: ['config'],
      operation: 'avg',
      summary: [],
    };
    assert.isEmpty(validatePivotRequest(req));
  });
});

describe('validateAsPivotTable', () => {
  it('returns error message on null request', () => {
    assert.isNotEmpty(validateAsPivotTable(null));
  });

  it('returns error message if summary is null', () => {
    const req: pivot.Request = {
      group_by: ['config'],
      operation: 'avg',
      summary: null,
    };
    assert.isNotEmpty(validateAsPivotTable(req));
  });

  it('returns error message if summary is empty', () => {
    const req: pivot.Request = {
      group_by: ['config'],
      operation: 'avg',
      summary: [],
    };
    assert.isNotEmpty(validateAsPivotTable(req));
  });

  it('returns no error message if request is valid and summary has at least one entry', () => {
    const req: pivot.Request = {
      group_by: ['config'],
      operation: 'avg',
      summary: ['sum'],
    };
    assert.isEmpty(validateAsPivotTable(req));
  });
});

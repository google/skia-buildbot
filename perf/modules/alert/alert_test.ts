import { assert } from 'chai';
import { validate } from './index';
import { Alert, SerializesToString } from '../json';

describe('alert/validate', () => {
  it('returns an error message for an empty query', () => {
    const alert: Alert = {
      id_as_string: '1',
      display_name: 'Alert',
      query: '',
      alert: 'admin@example.com',
      interesting: 0,
      bug_uri_template: '',
      algo: 'kmeans',
      step: 'cohen',
      state: 'ACTIVE',
      owner: 'admin@example.com',
      step_up_only: false,
      direction: 'BOTH',
      radius: 10,
      k: 50,
      group_by: '',
      sparse: false,
      minimum_num: 0,
      category: 'Experimental',
      action: 'noaction',
      issue_tracker_component: SerializesToString(''),
    };
    assert.equal(validate(alert), 'An alert must have a non-empty query.');
  });

  it('returns an empty string for a valid alert', () => {
    const alert: Alert = {
      id_as_string: '1',
      display_name: 'Alert',
      query: 'config=8888',
      alert: 'admin@example.com',
      interesting: 0,
      bug_uri_template: '',
      algo: 'kmeans',
      step: 'cohen',
      state: 'ACTIVE',
      owner: 'admin@example.com',
      step_up_only: false,
      direction: 'BOTH',
      radius: 10,
      k: 50,
      group_by: '',
      sparse: false,
      minimum_num: 0,
      category: 'Experimental',
      action: 'noaction',
      issue_tracker_component: SerializesToString(''),
    };
    assert.equal(validate(alert), '');
  });
});

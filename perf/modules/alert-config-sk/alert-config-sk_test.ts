import './index';
import { AlertConfigSk } from './alert-config-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import { Alert, SerializesToString } from '../json';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';

const awaitRender = () => new Promise((resolve) => requestAnimationFrame(resolve));

describe('alert-config-sk', () => {
  const newInstance = setUpElementUnderTest<AlertConfigSk>('alert-config-sk');

  let element: AlertConfigSk;
  beforeEach(() => {
    (window as any).perf = {
      radius: 7,
      interesting: 0.1,
      key_order: ['config'],
      notifications: 'none',
    };
    element = newInstance();
  });

  it('renders', async () => {
    assert.isNotNull(element);
    // Wait for initial render.
    await awaitRender();

    // Verify default display name from constructor.
    assert.equal(element.querySelector<HTMLInputElement>('#display-name')?.value, 'Name');
  });

  it('preserves zero values in config', async () => {
    element.config = {
      id_as_string: '-1',
      display_name: '',
      query: '',
      alert: '',
      interesting: 0,
      algo: 'kmeans',
      state: 'ACTIVE',
      owner: '',
      step_up_only: false,
      direction: 'BOTH',
      radius: 0,
      k: 50,
      group_by: '',
      sparse: false,
      minimum_num: 0,
      category: '',
      step: '',
      issue_tracker_component: '' as SerializesToString,
      bug_uri_template: '',
    };

    await awaitRender();

    // radius corresponds to #radius input
    assert.equal(element.querySelector<HTMLInputElement>('#radius')?.value, '0');
    // interesting corresponds to #threshold input
    assert.equal(element.querySelector<HTMLInputElement>('#threshold')?.value, '0');
  });

  it('populates fields from config', async () => {
    const config: Alert = {
      id_as_string: '123',
      display_name: 'My Alert',
      query: 'source_type=skia',
      alert: 'me@example.com',
      interesting: 50,
      algo: 'stepfit',
      state: 'ACTIVE',
      owner: 'me@example.com',
      step_up_only: false,
      direction: 'BOTH',
      radius: 15,
      k: 0,
      group_by: '',
      sparse: true,
      minimum_num: 1,
      category: 'Testing',
      step: '',
      action: 'report',
      issue_tracker_component: '12345' as SerializesToString,
      bug_uri_template: '',
    };
    element.config = config;

    // Wait for lit-html to render.
    await awaitRender();

    assert.equal(element.querySelector<HTMLInputElement>('#display-name')?.value, 'My Alert');
    assert.equal(element.querySelector<HTMLInputElement>('#owner')?.value, 'me@example.com');
    assert.equal(element.querySelector<HTMLInputElement>('#category')?.value, 'Testing');
    assert.equal(element.querySelector<HTMLInputElement>('#radius')?.value, '15');
    // 'interesting' in config maps to 'Threshold' input in UI.
    assert.equal(element.querySelector<HTMLInputElement>('#threshold')?.value, '50');
    assert.equal(element.querySelector<HTMLInputElement>('#k')?.value, '0');
    // 'minimum_num' in config maps to 'min' input in UI.
    assert.equal(element.querySelector<HTMLInputElement>('#min')?.value, '1');
    assert.isTrue(element.querySelector<CheckOrRadio>('#sparse')?.checked);

    // Verify query and algo
    assert.equal(element.querySelector('algo-select-sk')?.getAttribute('algo'), 'stepfit');
    assert.equal(
      element.querySelector('query-chooser-sk')?.getAttribute('current_query'),
      'source_type=skia'
    );
  });
});

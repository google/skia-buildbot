import './index';
import { assert } from 'chai';
import { DomainPickerSk } from './domain-picker-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const RANGE = 0;
const DENSE = 1;

describe('domain-picker-sk', () => {
  const newInstance = setUpElementUnderTest<DomainPickerSk>('domain-picker-sk');

  let element: DomainPickerSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('renders', () => {
    assert.isNotNull(element);
  });

  it('defaults to Range', () => {
    assert.equal(element.state.request_type, RANGE);
    // Range mode shows Begin date, Dense mode shows Points input
    assert.isNotNull(element.querySelector('.prefix')); // "Begin:" label
    assert.include(element.innerHTML, 'Begin:');
  });

  it('switches to Dense', () => {
    // Dispatch change event on the radio-sk element
    const radioDense = element.querySelector<HTMLElement>('radio-sk[label="Dense"]');
    assert.isNotNull(radioDense);
    radioDense!.dispatchEvent(new CustomEvent('change', { bubbles: true }));
    assert.equal(element.state.request_type, DENSE);
  });

  it('forces request type via attribute', () => {
    element.setAttribute('force_request_type', 'dense');
    assert.equal(element.state.request_type, DENSE);
    assert.isNull(element.querySelector('radio-sk'));
  });

  it('updates state', () => {
    element.state = {
      begin: 100,
      end: 200,
      num_commits: 99,
      request_type: DENSE,
    };
    assert.equal(element.state.begin, 100);
    assert.equal(element.state.end, 200);
    assert.equal(element.state.num_commits, 99);
    assert.equal(element.state.request_type, DENSE);
  });
});

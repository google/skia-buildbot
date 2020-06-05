import './index';

import { $$ } from 'common-sk/modules/dom';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('expandable-textarea-sk', () => {
  // Function to create a new expandable-textarea-sk.
  const newInstance = setUpElementUnderTest('expandable-textarea-sk');

  let et;
  beforeEach(() => {
    et = newInstance((el) => {
      el.displayText = 'Click to toggle';
    });
  });

  const simulateUserClick = () => {
    $$('a', et).click();
  };

  it('displays clickable text', () => {
    expect(et).to.not.have.attribute('open');
    expect($$('collapse-sk', et)).to.have.attribute('closed');
    expect(et).to.contain('expand-more-icon-sk').and.not.contain('expand-less-icon-sk');
    expect($$('a', et)).to.contain.text('Click to toggle');
  });

  it('expands on click with textarea in focus', () => {
    simulateUserClick();
    expect(et).to.have.attribute('open');
    expect($$('collapse-sk', et)).to.not.have.attribute('closed');
    expect(et).to.contain('expand-less-icon-sk').and.not.contain('expand-more-icon-sk');
    expect($$('textarea', et)).to.equal(document.activeElement);
  });

  it('collapses on second click', () => {
    simulateUserClick();
    simulateUserClick();
    expect(et).to.not.have.attribute('open');
    expect($$('collapse-sk', et)).to.have.attribute('closed');
    expect(et).to.contain('expand-more-icon-sk').and.not.contain('expand-less-icon-sk');
  });

  it('reflects textarea value', () => {
    $$('textarea', et).value = 'foo';
    expect(et).to.have.property('value', 'foo');
  });
});

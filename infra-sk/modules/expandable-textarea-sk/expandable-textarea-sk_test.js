import './index';

import { $$ } from 'common-sk/modules/dom';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('expandable-textarea-sk', () => {
  // Function to create a new expandable-textarea-sk.
  const newInstance = setUpElementUnderTest('expandable-textarea-sk');

  let expandableTextareaSk;
  beforeEach(() => {
    expandableTextareaSk = newInstance((el) => {
      el.displayText = 'Click to toggle';
    });
  });

  const simulateUserClick = () => {
    $$('button', expandableTextareaSk).click();
  };

  it('displays clickable text', () => {
    expect(expandableTextareaSk).to.not.have.attribute('open');
    expect($$('collapse-sk', expandableTextareaSk))
      .to.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-more-icon-sk')
      .and.not.contain('expand-less-icon-sk');
    expect($$('button', expandableTextareaSk)).to.contain.text('Click to toggle');
  });

  it('expands on click with textarea in focus', () => {
    simulateUserClick();
    expect(expandableTextareaSk).to.have.attribute('open');
    expect($$('collapse-sk', expandableTextareaSk))
      .to.not.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-less-icon-sk')
      .and.not.contain('expand-more-icon-sk');
    expect($$('textarea', expandableTextareaSk))
      .to.equal(document.activeElement);
  });

  it('collapses on second click', () => {
    simulateUserClick();
    simulateUserClick();
    expect(expandableTextareaSk).to.not.have.attribute('open');
    expect($$('collapse-sk', expandableTextareaSk))
      .to.have.attribute('closed');
    expect(expandableTextareaSk)
      .to.contain('expand-more-icon-sk')
      .and.not.contain('expand-less-icon-sk');
  });

  it('reflects textarea value', () => {
    $$('textarea', expandableTextareaSk).value = 'foo';
    expect(expandableTextareaSk).to.have.property('value', 'foo');
  });
});

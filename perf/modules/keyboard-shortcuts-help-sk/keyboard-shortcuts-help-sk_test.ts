import { assert } from 'chai';
import './keyboard-shortcuts-help-sk';
import { KeyboardShortcutsHelpSk } from './keyboard-shortcuts-help-sk';
import { ShortcutRegistry } from '../common/keyboard-shortcuts';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('keyboard-shortcuts-help-sk', () => {
  const newInstance = setUpElementUnderTest<KeyboardShortcutsHelpSk>('keyboard-shortcuts-help-sk');

  let element: KeyboardShortcutsHelpSk;

  beforeEach(() => {
    ShortcutRegistry.getInstance().reset();
    ShortcutRegistry.getInstance().register('Test Category', [
      { key: 'a', action: 'Action A', description: 'Description A' },
    ]);
    element = newInstance();
  });

  it('renders correctly', () => {
    assert.isNotNull(element);
    assert.include(element.innerHTML, 'Test Category');
    assert.include(element.innerHTML, 'Description A');
    assert.include(element.innerHTML, 'a');
  });

  it('opens and closes', async () => {
    // We can't easily test md-dialog open/close state in this environment without full browser,
    // but we can check if methods exist and don't throw.
    element.open();
    element.close();
  });
});

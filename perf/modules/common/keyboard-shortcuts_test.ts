import { assert } from 'chai';
import { ShortcutRegistry, Shortcut } from './keyboard-shortcuts';

describe('ShortcutRegistry', () => {
  beforeEach(() => {
    ShortcutRegistry.getInstance().reset();
  });

  it('should be a singleton', () => {
    const instance1 = ShortcutRegistry.getInstance();
    const instance2 = ShortcutRegistry.getInstance();
    assert.strictEqual(instance1, instance2);
  });

  it('should register and retrieve shortcuts', () => {
    const registry = ShortcutRegistry.getInstance();
    const shortcuts: Shortcut[] = [{ key: 'a', action: 'Action A', description: 'Description A' }];
    registry.register('Category A', shortcuts);

    const retrieved = registry.getShortcuts();
    assert.isTrue(retrieved.has('Category A'));
    assert.deepEqual(retrieved.get('Category A'), shortcuts);
    assert.isTrue(retrieved.size > 1);
  });

  it('should reset shortcuts', () => {
    const registry = ShortcutRegistry.getInstance();
    registry.register('Category A', []);
    registry.reset();
    assert.equal(registry.getShortcuts().size, 4); // Triage, Navigation, Report, General
  });
});

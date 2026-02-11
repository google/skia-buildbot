import { ReactiveController, ReactiveControllerHost } from 'lit';

export class SelectionController<T> implements ReactiveController {
  private host: ReactiveControllerHost;

  private selection: Set<T> = new Set<T>();

  constructor(host: ReactiveControllerHost) {
    this.host = host;
    this.host.addController(this);
  }

  hostConnected(): void {}

  hostDisconnected(): void {}

  /**
   * Clears the selection.
   */
  clear(): void {
    this.selection.clear();
    this.host.requestUpdate();
  }

  /**
   * Adds an item to the selection.
   */
  select(item: T): void {
    this.selection.add(item);
    this.host.requestUpdate();
  }

  /**
   * Removes an item from the selection.
   */
  deselect(item: T): void {
    this.selection.delete(item);
    this.host.requestUpdate();
  }

  /**
   * Toggles the selection state of an item.
   * @param item The item to toggle.
   * @param checked Optional explicit state. If provided, sets the state to this value.
   */
  toggle(item: T, checked?: boolean): void {
    const isSelected = this.selection.has(item);
    const shouldSelect = checked !== undefined ? checked : !isSelected;

    if (shouldSelect) {
      this.selection.add(item);
    } else {
      this.selection.delete(item);
    }
    this.host.requestUpdate();
  }

  /**
   * Checks if an item is selected.
   */
  has(item: T): boolean {
    return this.selection.has(item);
  }

  /**
   * Returns the size of the selection.
   */
  get size(): number {
    return this.selection.size;
  }

  /**
   * Returns the selection as an array.
   */
  get items(): T[] {
    return Array.from(this.selection);
  }
}

import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';
import { Commit } from '../json';
import { CommitDetailPanelSk } from './commit-detail-panel-sk';

// Define which properties of the component
// are readable/writable by the page object.
type WritableProps = 'details' | 'selectable' | 'selected' | 'hide';
type ReadableProps = WritableProps; // In this case, they are the same.

/** A page object for the CommitDetailPanelSk component. */
export class CommitDetailPanelSkPO extends PageObject {
  private get table(): PageObjectElement {
    return this.bySelector('table');
  }

  private get rows(): PageObjectElementList {
    return this.bySelectorAll('tr');
  }

  async getRowCount(): Promise<number> {
    return this.rows.length;
  }

  async clickRow(index: number): Promise<void> {
    const row = await this.rows.item(index);
    await row.click();
  }

  // Helper to get a property from the underlying component.
  private _getProperty<K extends ReadableProps>(propertyName: K): Promise<CommitDetailPanelSk[K]> {
    return this.element.applyFnToDOMNode(
      (el, name) => (el as CommitDetailPanelSk)[name as K],
      propertyName
    );
  }

  // Helper to set a property on the underlying component.
  private _setProperty<K extends WritableProps>(
    propertyName: K,
    value: CommitDetailPanelSk[K]
  ): Promise<void> {
    return this.element.applyFnToDOMNode(
      (el, name, val) => {
        (el as any)[name as K] = val;
      },
      propertyName,
      value
    );
  }

  async isSelectable(): Promise<boolean> {
    return this._getProperty('selectable');
  }

  async setSelectable(selectable: boolean): Promise<void> {
    await this._setProperty('selectable', selectable);
  }

  async getSelectedRow(): Promise<number> {
    return this._getProperty('selected');
  }

  async setSelectedRow(index: number): Promise<void> {
    await this._setProperty('selected', index);
  }

  async isHidden(): Promise<boolean> {
    return this._getProperty('hide');
  }

  async setHidden(hidden: boolean): Promise<void> {
    await this._setProperty('hide', hidden);
  }

  async getDetails(): Promise<Commit[]> {
    return this._getProperty('details');
  }

  async setDetails(details: Commit[]): Promise<void> {
    await this._setProperty('details', details);
  }
}

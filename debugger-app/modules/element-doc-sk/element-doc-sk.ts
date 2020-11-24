/**
 * @module modules/element-doc-sk
 * @description Sub class of ElementSK that remembers and cleans up document event listeners.
 * Extended by all modules of debuugger. TODO(nifong): merge into ElementSk
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class ElementDocSk extends ElementSk {

  constructor(templateFn?: (el: any) => unknown) {
    super(templateFn);
  }

  private _documentEventListeners = new Map<string, (e: Event)=>void>();

  disconnectedCallback() {
    for (const [key, val] of this._documentEventListeners.entries()) {
      document.removeEventListener(key, val);
    }
    super.disconnectedCallback();
  }

  addDocumentEventListener(name: string, fn: (e: any)=>void, useCapture: boolean = false) {
    this._documentEventListeners.set(name, fn);
    document.addEventListener(name, fn, useCapture);
  }
};

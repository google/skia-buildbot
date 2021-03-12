import './index';
import { NoteEditorSk } from './note-editor-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('note-editor-sk', () => {
  const newInstance = setUpElementUnderTest<NoteEditorSk>('note-editor-sk');

  let element: NoteEditorSk;
  beforeEach(() => {
    element = newInstance((el: NoteEditorSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});

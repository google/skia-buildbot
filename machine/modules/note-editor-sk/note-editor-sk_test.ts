import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { NoteEditorSk } from './note-editor-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Annotation } from '../json';

const start: Annotation = {
  Message: 'This is a test.',
  User: '',
  Timestamp: new Date(Date.now()).toString(),
};

describe('note-editor-sk', () => {
  const newInstance = setUpElementUnderTest<NoteEditorSk>('note-editor-sk');

  let element: NoteEditorSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('note-editor-sk', () => {
    it('returns undefined on cancel', async () => {
      const promise = element.edit(start);
      $$<HTMLButtonElement>('#cancel', element)!.click();
      assert.isUndefined(await promise);
    });
  });
});

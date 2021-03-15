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

  it('returns undefined on cancel', async () => {
    const promise = element.edit(start);
      $$<HTMLButtonElement>('#cancel', element)!.click();
      assert.isUndefined(await promise);
  });

  it('returns a modified Message on OK', async () => {
    const modifiedString = 'This is an edited message.';
    const promise = element.edit(start);
      $$<HTMLInputElement>('#note', element)!.value = modifiedString;
      $$<HTMLButtonElement>('#ok', element)!.click();
      const note = await promise;
      assert.equal(note?.Message, modifiedString);
  });

  it('returns an empty Message on Clear', async () => {
    const promise = element.edit(start);
      $$<HTMLButtonElement>('#clear', element)!.click();
      const note = await promise;
      assert.isEmpty(note!.Message);
  });
});

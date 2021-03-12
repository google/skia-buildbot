import { $$ } from 'common-sk/modules/dom';
import { Annotation } from '../json';
import './index';
import { NoteEditorSk } from './note-editor-sk';

const start: Annotation = {
  Message: 'Hello World',
  User: '',
  Timestamp: new Date(Date.now()).toString(),
};

const editButton = $$<HTMLButtonElement>('#edit')!;

editButton.addEventListener('click', async () => {
  const results = await $$<NoteEditorSk>('note-editor-sk')!.edit(start);
  $$<HTMLPreElement>('#results')!.textContent = JSON.stringify(results);
});

editButton.click();

/**
 * @fileoverview Mock data for testing PickerFieldSk component.
 */

export const DEFAULT_LABEL: string = 'benchmark';

export const DEFAULT_OPTIONS: string[] = ['V8', 'V8 Infra'];

export const DEFAULT_SELECTED_ITEMS: string[] = ['V8'];

export const DEFAULT_HELPER_TEXT: string = 'Select one or more options';

export const NEW_LABEL: string = 'benchmark/bot';

export const NEW_OPTIONS: string[] = Array.from(
  { length: 3000 },
  (_, index) => `speedometer${index + 1}`
);

export const NEW_SELECTED_ITEMS: string[] = ['V8', 'speedometer3'];

// Declaration for the window.sk global variable.
import { SkPerfConfig } from '../json/all';

declare global {
  interface Window {
      perf: SkPerfConfig;
  }
}

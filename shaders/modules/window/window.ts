// Declaration for the window.sk global variable.
import { SkShadersConfig } from '../json';

declare global {
  interface Window {
    shaders: SkShadersConfig;
  }
}

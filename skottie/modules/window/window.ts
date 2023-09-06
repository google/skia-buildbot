// Declaration for the window.sk global variable.
import { SkSkottieConfig } from '../json';

declare global {
  interface Window {
    skottie: SkSkottieConfig;
  }
}

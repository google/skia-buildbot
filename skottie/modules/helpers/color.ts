/**
 * helper functions for color conversions
 *
 */

function componentToHex(c: number): string {
  c = Math.round(c);
  const hex = c.toString(16);
  return hex.length === 1 ? `0${hex}` : hex;
}

function rgbToHex(r: number, g: number, b: number): string {
  return `#${componentToHex(r)}${componentToHex(g)}${componentToHex(b)}`;
}

function hexToRgb(hex: string): number[] {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  return result
    ? [parseInt(result[1], 16), parseInt(result[2], 16), parseInt(result[3], 16)]
    : [0, 0, 0];
}

function hexToColor(value: string): number[] {
  const rgb = hexToRgb(value);
  return [rgb[0] / 255, rgb[1] / 255, rgb[2] / 255];
}

function colorToHex(color: number[]): string {
  if (color.length > 2) {
    return rgbToHex(color[0] * 255, color[1] * 255, color[2] * 255);
  }
  return '#FF0000';
}

export { hexToColor, colorToHex };

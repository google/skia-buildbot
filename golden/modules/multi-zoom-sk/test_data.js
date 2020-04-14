// The three base64-encoded png images here were created with the following code snippets. It was
// designed to have some pixels with nothing different, some pixels that differed just in alpha
// and some pixels that differed in all channels. This is robust enough for testing the navigation
// controls of the multi-zoom-sk element.

// Assumes canvas is an html element that is 16x16.
//   const ctx = canvas.getContext('2d');
//   ctx.imageSmoothingEnabled = false;
//   ctx.clearRect(0, 0, 16, 16);
//   ctx.lineWidth = 3;
//   ctx.strokeStyle = 'black';
//   ctx.beginPath();
//   ctx.moveTo(3, 1);
//   ctx.lineTo(4, 15);
//   ctx.stroke();
//
//   ctx.fillStyle = 'rgba(255, 10, 10, 0.5)';
//   ctx.fillRect(12, 0, 5, 5);
//
//   console.log(canvas.toDataURL());

export const left16x16 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAA10lEQVQ4T5XTt02EQRBA4e9CfohIMR1gOsB0gOkA08GJEC6EEjAdYDrAdIDpAJMScRCCBs0hOHHSzUQ75j1pd7Qtf2ME43jpq/+kn02z97vXymQHuwjBASL/NwYJtnCYxAk2q4I1nCZ0gdWqYAmXCd1gsSqYw21CD5gdJOiv9x5xCo/ZjA1MVgWjeEvoA01VEPPvucY4j6E7jKR3hZh9xkRC03iqCu4xk9A87qqCaywktIyrquAcKwmt46wqOMZGQts4qgriE7URa+xgvyqIDbymYBj2e+YL11gn8FDepQMAAAAASUVORK5CYII=';

//   ctx.clearRect(0, 0, 16, 16);
//   ctx.lineWidth = 3;
//   ctx.strokeStyle = 'black';
//   ctx.beginPath();
//   ctx.moveTo(3, 1);
//   ctx.lineTo(4, 15);
//   ctx.stroke();
//
//   ctx.fillStyle = 'rgba(255, 10, 10, 0.8)'; // only alpha differs
//   ctx.fillRect(12, 0, 5, 5);
//
//   ctx.fillStyle = 'rgba(255, 10, 10, 0.8)'; // all channels differ from transparent black
//   ctx.fillRect(12, 13, 5, 5);
//
//   console.log(canvas.toDataURL());

export const right16x16 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAA3UlEQVQ4T5XTt01FQRBA0fNDHkSkmA4wHWA6wHSA6QARIkIoAdMBpgNMToDpAJMSYULQSPMRPP0nMRvtzM69mt3R9vxdQxjFSyv/E341zc3vs14G29hBCPYR8cDVJdjAQRLHWK8KVnCS0DmWq4IFXCR0jfmqYAa3CT1gukvQzvcfcQKPeRgTGK8KhvGW0CeaqiDqP3KMsR/B+38k/StE7TPGEprEU1Vwj6mEZnFXFVxhLqFFXFYFZ1hKaBWnVcER1hLaxOEgQddfiNr4RFuIMe5iryqICbymoLP7dgffkIAvCZYHkAAAAAAASUVORK5CYII=';

// Manually draw the diff ourselves. The diff is just drawn, it isn't used for calculating which
// pixels are different; that's done directly from the inputs.

//   ctx.clearRect(0, 0, 16, 16);
//   ctx.fillStyle = '#2171b5'; // this blue color means just alpha differs
//   ctx.fillRect(12, 0, 5, 5);
//
//   ctx.fillStyle = '#7f2704'; // this dark orange means multiple channels differ by a lot.
//   ctx.fillRect(12, 13, 5, 5);
//
//   console.log(canvas.toDataURL());

export const diff16x16 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAMElEQVQ4T2NkIBEoFm79j6yFkUT9DKMGYIYYyYGIbsSoAQwMQzAM6tVZKMtMVDcAANW2EGoA8ciVAAAAAElFTkSuQmCC';

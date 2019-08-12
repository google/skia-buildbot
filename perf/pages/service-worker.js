importScripts('https://storage.googleapis.com/workbox-cdn/releases/4.3.1/workbox-sw.js');

workbox.precaching.cleanupOutdatedCaches();

workbox.precaching.precacheAndRoute([
  {
    "url": "/offline",
    "revision": "2"
  },
]);

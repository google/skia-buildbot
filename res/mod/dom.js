// A Promise that resolves when DOMContentLoaded has fired.
export const DomReady = new Promise(function(resolve, reject) {
  if (document.readyState !== 'loading') {
    // If readyState is already past loading then
    // DOMContentLoaded has already fired, so just resolve.
    resolve();
  } else {
    document.addEventListener('DOMContentLoaded', resolve);
  }
});

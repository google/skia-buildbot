// errorMessage dispatches an event with the error message in it. The value of
// 'message' is expected to be an object with either a field response (e.g.
// server response) or message (e.g. message of a typeError) that is a String.
//
// Use this function in conjunction with the <error-toast-sk> element.
export function errorMessage(message, duration) {
  if (typeof message === 'object') {
    message = message.message        || // for handling Errors {name:String, message:String}
              JSON.stringify(message);  // for everything else
  }
  var detail = {
    message: message,
    duration: duration,
  }
  document.dispatchEvent(new CustomEvent('error-sk', {detail: detail, bubbles: true}));
}

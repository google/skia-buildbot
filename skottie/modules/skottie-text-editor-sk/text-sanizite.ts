const LINE_FEED = 10;
const FORM_FEED = 13;

const sanitizeText = (text: string): string => {
  let sanitizedText = '';
  for (let i = 0; i < text.length; i += 1) {
    if (text.charCodeAt(i) === LINE_FEED) {
      sanitizedText += String.fromCharCode(FORM_FEED);
    } else {
      sanitizedText += text.charAt(i);
    }
  }
  return sanitizedText;
};

export default sanitizeText;

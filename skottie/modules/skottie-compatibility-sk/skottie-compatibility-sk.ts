/**
 * @module skottie-compatibility-sk
 * @description <h2><code>skottie-compatibility-sk</code></h2>
 *
 * <p>
 *   A skottie compatibility report. Reports the input lottie with various
 * JSON schemas.
 * </p>
 */
import { html } from 'lit-html';
import Ajv, { ErrorObject } from 'ajv/dist/2020';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';
import { schema } from './schemas/lottie.schema';

export class SkottieCompatibilitySk extends ElementSk {
  private ajv = new Ajv();

  public validate = this.ajv.compile(schema);

  private _animation: LottieAnimation | null = null;

  constructor() {
    super(SkottieCompatibilitySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }

  private static template = (ele: SkottieCompatibilitySk) => html`
    <div>
      <table>
        <tr>
          <th>Name</th>
          <th>Result${this.downloadLinkElement(ele)}</th>
        </tr>
        <tr>
          <td>Lottie Specification</td>
          <td>${this.resultElement(ele)}</td>
        </tr>
      </table>
    </div>
  `;

  private static resultElement = (ele: SkottieCompatibilitySk) => {
    if (ele.validate.errors && ele.validate.errors.length > 0) {
      return html`<table>
        ${processErrorMap(ele.validate.errors).map(
          (error) =>
            html`<tr>
              <td>${error.instancePath}</td>
              <td>${error.message}</td>
            </tr>`
        )}
      </table>`;
    }

    return html`<div>Pass</div>`;
  };

  private static downloadLinkElement = (ele: SkottieCompatibilitySk) => {
    if ((ele.validate.errors?.length ?? 0) === 0) {
      return null;
    }

    return html`
      <span>
        -
        <a
          download="${ele._animation?.nm ?? 'lottie'}_spec-errors.json"
          href=${`data:text/plain;charset=utf-8,${encodeURIComponent(
            JSON.stringify(ele.validate.errors)
          )}`}>
          Download
        </a>
      </span>
    `;
  };

  set animation(val: LottieAnimation) {
    if (this._animation !== val) {
      this._animation = val;
      // Errors will be populated to this.validate object
      this.validate(val);
      this._render();
    }
  }
}

/**
 * Process errors returned from ajv.
 *
 *  1. Remove errors with duplicated <path + message>
 *  2. Sort in path ascending order
 */
function processErrorMap(errors: ErrorObject[]): ErrorObject[] {
  const uniqueErrorMap = errors.reduce((errorMap, err) => {
    const key = `${err.instancePath}:${err.message}`;

    if (!errorMap.has(key)) {
      errorMap.set(key, err);
    }

    return errorMap;
  }, new Map<string, ErrorObject>());

  return [...uniqueErrorMap.values()].sort((e1, e2) =>
    e1.instancePath.localeCompare(e2.instancePath)
  );
}

define('skottie-compatibility-sk', SkottieCompatibilitySk);

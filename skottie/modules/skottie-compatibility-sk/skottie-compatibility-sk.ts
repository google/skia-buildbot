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
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';
import { ProfileValidator, LottieError } from './profile-validator';
import { lottieSchema } from './schemas/lottie.schema';
import { lowPowerLottieProfileSchema } from './schemas/low-power-lottie-profile.schema';

type SchemaEntry = {
  name: string;
  validator: ProfileValidator;
  featureErrorsOnly?: boolean;
};

export class SkottieCompatibilitySk extends ElementSk {
  private _animation: LottieAnimation | null = null;

  private schemas: SchemaEntry[] = [];

  constructor() {
    super(SkottieCompatibilitySk.template);

    this.schemas = [
      {
        name: 'Lottie Specfication',
        validator: new ProfileValidator(lottieSchema),
      },
      {
        name: 'Low Power Profile',
        validator: new ProfileValidator(lowPowerLottieProfileSchema),
        featureErrorsOnly: true,
      },
    ];
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
          <th>Result</th>
        </tr>
        ${this.report(ele)}
      </table>
    </div>
  `;

  private static report = (ele: SkottieCompatibilitySk) =>
    ele.schemas.map(
      (schema) => html`
        <tr>
          <td class="report-name-cell">
            <div>${schema.name}</div>
            ${this.downloadLinkElement(ele, schema)}
          </td>
          ${this.resultCell(schema, ele._animation)}
        </tr>
      `
    );

  private static resultCell = (schemaEntry: SchemaEntry, lottie: any) => {
    const errors = schemaEntry.validator.getErrors(
      lottie,
      schemaEntry.featureErrorsOnly
    );
    if (errors && errors.length > 0) {
      return html`<td class="result-cell">
        <table class="result-cell-table">
          ${errors.map(
            (error) =>
              html`<tr>
                <td>
                  ${`${error.nameHierarchy?.join(' > ')} (${
                    error.instancePath
                  })`}
                </td>
                ${this.getErrorMessageCell(error)}
              </tr>`
          )}
        </table>
      </td>`;
    }

    return html`<td>Pass</td>`;
  };

  private static getErrorMessageCell(error: LottieError) {
    if (error.featureCode) {
      return html`<td>
        <a href="https://canilottie.com/${error.featureCode}">${error.featureCode}</a> not supported
      </td`;
    }

    return html`<td>${error.message}</div>`;
  }

  private static downloadLinkElement = (
    ele: SkottieCompatibilitySk,
    schemaEntry: SchemaEntry
  ) => {
    const errors = schemaEntry.validator.getErrors(
      ele._animation,
      schemaEntry.featureErrorsOnly
    );

    if ((errors.length ?? 0) === 0) {
      return null;
    }

    return html`
      <div>
        <a
          download="${ele._animation?.nm ?? 'lottie'}_spec-errors.json"
          href=${`data:text/plain;charset=utf-8,${encodeURIComponent(
            JSON.stringify(errors)
          )}`}>
          Download Report
        </a>
      </div>
    `;
  };

  set animation(val: LottieAnimation) {
    if (this._animation !== val) {
      this._animation = val;

      // Errors are persisted in each validator object
      this.schemas.forEach((schema) => {
        schema.validator.validate(this._animation);
      });

      this._render();
    }
  }
}

define('skottie-compatibility-sk', SkottieCompatibilitySk);

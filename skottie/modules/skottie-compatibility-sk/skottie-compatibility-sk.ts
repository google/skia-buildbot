/**
 * @module skottie-compatibility-sk
 * @description <h2><code>skottie-compatibility-sk</code></h2>
 *
 * <p>
 *   A skottie compatibility report. Reports the input lottie with various
 * JSON schemas.
 * </p>
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';
import '../skottie-button-sk';
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

  tabIndex = 0;

  constructor() {
    super(SkottieCompatibilitySk.template);

    this.schemas = [
      {
        name: 'Low Power Profile (WIP)',
        validator: new ProfileValidator(lowPowerLottieProfileSchema),
        featureErrorsOnly: true,
      },
      {
        name: 'Lottie Specfication 1.0 (WIP)',
        validator: new ProfileValidator(lottieSchema),
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

  setTabIndex(tabIndex: number) {
    this.tabIndex = tabIndex;
    this._render();
  }

  private static tabContainer = (ele: SkottieCompatibilitySk) => html`
    <div class="tab-container">
      ${ele.schemas.map((schema, index) => {
        const errors = schema.validator.getErrors(
          ele._animation,
          schema.featureErrorsOnly
        );

        const resultSummary =
          errors.length === 0 ? 'Pass' : `Fail (${errors.length})`;

        return html`
          <skottie-button-sk
            id="Report-${index}"
            @select=${() => ele.setTabIndex(index)}
            type="outline"
            .content=${`${schema.name} - ${resultSummary}`}
            .classes=${ele.tabIndex === index ? ['active-tab'] : []}>
          </skottie-button-sk>
        `;
      })}
    </div>
  `;

  private static template = (ele: SkottieCompatibilitySk) => html`
    <div>${this.tabContainer(ele)}</div>
    <div>${this.report(ele)}</div>
  `;

  private static report = (ele: SkottieCompatibilitySk) => {
    const tabIndex = ele.tabIndex;
    const schema = ele.schemas[tabIndex];
    const lottie = ele._animation;

    const errors = schema.validator.getErrors(lottie, schema.featureErrorsOnly);

    if (errors.length === 0) {
      return html`<div class="pass-message">Pass</div>`;
    }

    if (schema.featureErrorsOnly) {
      return this.featureErrorReport(schema, ele._animation);
    }

    return this.schemaReport(schema, ele._animation);
  };

  private static schemaReport(schema: SchemaEntry, lottie: any) {
    const errors = schema.validator.getErrors(lottie, schema.featureErrorsOnly);

    return html`
      <table>
        <tr>
          <th>Element Name</th>
          <th>JSON Path</th>
          <th>Error</th>
          <th>Details</th>
          <th>Schema Path</th>
        </tr>
        ${errors.map(
          (error) =>
            html`<tr>
              <td>${`${error.nameHierarchy?.join(' > ')}`}</td>
              <td>${error.instancePath}</td>
              <td>${error.message}</td>
              <td>${JSON.stringify(error.params)}</td>
              <td>${error.schemaPath}</td>
            </tr>`
        )}
      </table>
    `;
  }

  private static featureErrorReport = (
    schemaEntry: SchemaEntry,
    lottie: any
  ) => {
    const errors = schemaEntry.validator.getErrors(
      lottie,
      schemaEntry.featureErrorsOnly
    );

    const featureToErrorList: Map<string, LottieError[]> = errors.reduce(
      (map, error) => {
        const { featureCode } = error;

        if (!featureCode) {
          return map;
        }

        if (!map.get(featureCode)) {
          map.set(featureCode, []);
        }

        map.get(featureCode)?.push(error);

        return map;
      },
      new Map<string, LottieError[]>()
    );

    return html`<table>
      <tr>
        <th>Feature ID</th>
        <th>Element Name</th>
        <th>JSON Path</th>
      </tr>
      ${[...featureToErrorList].map(([featureCode, errorList]) =>
        errorList.map(
          (error, index) => html`
            <tr>
              ${index === 0
                ? html`<td class="feature-id-cell" rowspan=${errorList.length}>
                    <a
                      href="https://canilottie.com/${error.featureLink ??
                      error.featureCode}"
                      >${error.featureCode}</a
                    >
                    ${error.featureLevel === 'partial'
                      ? 'partially supported'
                      : 'not supported'}
                    ${error.featureDetails
                      ? html` <div>${error.featureDetails}</div> `
                      : null}
                  </td>`
                : null}
              <td>${error.nameHierarchy?.join(' > ')}</td>
              <td>${error.instancePath}</td>
            </tr>
          `
        )
      )}
    </table>`;
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

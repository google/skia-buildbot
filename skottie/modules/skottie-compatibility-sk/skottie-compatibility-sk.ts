/**
 * @module skottie-compatibility-sk
 * @description <h2><code>skottie-compatibility-sk</code></h2>
 *
 * <p>
 *   A skottie compatibility report. Reports the input lottie with various
 * JSON schemas.
 * </p>
 */
import Ajv from 'ajv/dist/2020';
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';
import '../skottie-button-sk';
import { ProfileValidator, LottieError } from './profile-validator';
import { lottieSchema } from './schemas/lottie.schema';
import { lottiePerformanceWarningSchema } from './schemas/lottie-performance-warning.schema';
import { lowPowerLottieProfileSchema } from './schemas/low-power-lottie-profile.schema';
import {
  LottieValidator,
  LottieValidatorError,
} from '@lottie-animation-community/lottie-specs/src/validator';

type SchemaEntry = {
  name: string;
  getContent: (ele: SkottieCompatibilitySk) => TemplateResult;
  getErrorCount?: (ele: SkottieCompatibilitySk) => number;
  profileValidator?: ProfileValidator;
};

export class SkottieCompatibilitySk extends ElementSk {
  private _animation: LottieAnimation | null = null;

  private schemas: SchemaEntry[] = [];

  tabIndex = 0;

  private specValidator: LottieValidator;

  private specErrors: LottieValidatorError[] = [];

  constructor() {
    super(SkottieCompatibilitySk.template);

    this.specValidator = new LottieValidator(Ajv, lottieSchema, { name_paths: true });

    const lowPowerProfile = new ProfileValidator(lowPowerLottieProfileSchema);
    const lowPowerWarningProfile = new ProfileValidator(lottiePerformanceWarningSchema);

    this.schemas = [
      {
        name: 'Specification 1.0 Errors',
        getContent: (ele) => SkottieCompatibilitySk.specReport(ele, 'error'),
        getErrorCount: (ele) => SkottieCompatibilitySk.specErrorCount(ele, 'error'),
      },
      {
        name: 'Specification 1.0 Warnings',
        getContent: (ele) => SkottieCompatibilitySk.strictSpecReport(ele, 'warning'),
        getErrorCount: (ele) => SkottieCompatibilitySk.specErrorCount(ele, 'warning'),
      },
      {
        name: 'Low Power Profile Errors (WIP)',
        getContent: (ele) =>
          SkottieCompatibilitySk.featureErrorReport(lowPowerProfile, ele._animation),
        getErrorCount: (ele) => lowPowerProfile.getErrors(ele._animation, true).length,
        profileValidator: lowPowerProfile,
      },
      {
        name: 'Low Power Profile Performance Warnings (WIP)',
        getContent: (ele) =>
          SkottieCompatibilitySk.featureErrorReport(lowPowerWarningProfile, ele._animation),
        getErrorCount: (ele) => lowPowerWarningProfile.getErrors(ele._animation, true).length,
        profileValidator: lowPowerWarningProfile,
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
        let resultSummary = 'Unknown';
        if (schema.getErrorCount) {
          const errorCount = schema.getErrorCount(ele);
          resultSummary = errorCount === 0 ? 'Pass' : `${errorCount}`;
        }

        return html`
          <skottie-button-sk
            id="Report-${index}"
            @select=${() => ele.setTabIndex(index)}
            type="outline"
            .content=${`${schema.name} (${resultSummary})`}
            .classes=${ele.tabIndex === index ? ['active-tab'] : []}>
          </skottie-button-sk>
        `;
      })}
    </div>
  `;

  private static template = (ele: SkottieCompatibilitySk) => html`
    <div>${this.tabContainer(ele)}</div>
    <div class="tab-content">${this.report(ele)}</div>
  `;

  private static report = (ele: SkottieCompatibilitySk) => {
    const tabIndex = ele.tabIndex;
    const schema = ele.schemas[tabIndex];

    return schema.getContent(ele);
  };

  private static specErrorCount(ele: SkottieCompatibilitySk, typeSelector?: string) {
    const filteredErrors = typeSelector
      ? ele.specErrors.filter((error) => error.type === typeSelector)
      : ele.specErrors;
    return filteredErrors.length;
  }

  private static specReport(ele: SkottieCompatibilitySk, typeSelector?: string) {
    const filteredErrors = typeSelector
      ? ele.specErrors.filter((error) => error.type === typeSelector)
      : ele.specErrors;

    if (filteredErrors.length === 0) {
      return html`<div>Pass</div>`;
    }

    return html`
      <table>
        <tr>
          <th>Element Name</th>
          <th>JSON Path</th>
          <th>Error</th>
        </tr>
        ${filteredErrors.map(
          (error) =>
            html`<tr>
              <td>${`${error.path_names?.join(' > ')}`}</td>
              <td>${error.path}</td>
              <td>${error.message}</td>
            </tr>`
        )}
      </table>
    `;
  }

  private static strictSpecReport(ele: SkottieCompatibilitySk, typeSelector?: string) {
    const filteredErrors = typeSelector
      ? ele.specErrors.filter((error) => error.type === typeSelector)
      : ele.specErrors;

    if (filteredErrors.length === 0) {
      return html`<div>Pass</div>`;
    }

    return html`
      <div class="strict-info">
        Specification warnings are for properties and types that have not been documented in the
        official Lottie specification. Exporter tools and playback libraries may still support these
        without any issues.
      </div>
      ${SkottieCompatibilitySk.specReport(ele, typeSelector)}
    `;
  }

  private static featureErrorReport = (profileValidator: ProfileValidator, lottie: any) => {
    const errors = profileValidator.getErrors(lottie, true);

    if (errors.length === 0) {
      return html`<div>Pass</div>`;
    }

    const featureToErrorList: Map<string, LottieError[]> = errors.reduce((map, error) => {
      const { featureCode } = error;

      if (!featureCode) {
        return map;
      }

      if (!map.get(featureCode)) {
        map.set(featureCode, []);
      }

      map.get(featureCode)?.push(error);

      return map;
    }, new Map<string, LottieError[]>());

    return html`<table>
      <tr>
        <th>Feature ID</th>
        <th>Element Name</th>
        <th>JSON Path</th>
      </tr>
      ${[...featureToErrorList].map(([, errorList]) =>
        errorList.map(
          (error, index) => html`
            <tr>
              ${index === 0
                ? html`<td class="feature-id-cell" rowspan=${errorList.length}>
                    <a href="https://canilottie.com/${error.featureLink ?? error.featureCode}"
                      >${error.featureCode}</a
                    >
                    ${error.featureDetails ? error.featureDetails : 'not supported'}
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
      this.specErrors = this.specValidator.validate(this._animation);
      // Errors are persisted in each validator object
      this.schemas.forEach((schema) => {
        schema.profileValidator?.validate(this._animation);
      });

      this._render();
    }
  }
}

define('skottie-compatibility-sk', SkottieCompatibilitySk);

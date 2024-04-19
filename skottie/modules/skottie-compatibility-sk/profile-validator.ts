/**
 * Wrapper around ajv library for JSON schema validation that includes
 * additional funcality for processing lotties
 */

/* eslint-disable import/no-unresolved */
import Ajv, { ErrorObject, ValidateFunction } from 'ajv/dist/2020';

export interface LottieError extends ErrorObject {
  featureCode?: string;
  nameHierarchy?: string[];
}

const FEATURE_CODE_KEYWORD = 'feature-code';

const NAME_PROPERTY_PATH = 'nm';

export class ProfileValidator {
  private validateFunction: ValidateFunction;

  constructor(profileSchema: any) {
    const ajv = new Ajv({
      allErrors: true,
    });

    ajv.addKeyword({
      keyword: FEATURE_CODE_KEYWORD,
      schemaType: 'string',
    });

    this.validateFunction = ajv.compile(profileSchema);
  }

  validate(lottie: any) {
    return this.validateFunction(lottie);
  }

  // Assumes validate() has already been called
  getErrors(lottie: any, featureErrorsOnly = false) {
    return processErrors(this.validateFunction, lottie, featureErrorsOnly);
  }
}

/**
 * Process errors returned from ajv.
 *
 *  1. Remove errors with duplicated <path + message>
 *  2. Sort in path ascending order
 */
function processErrors(
  validate: ValidateFunction,
  lottie: any,
  featureErrorsOnly = false
): LottieError[] {
  const errors = validate.errors;

  if (!errors) {
    return [];
  }

  const enhancedErrors = errors.map((error: LottieError) => {
    const featureCode = getSchemaPathFeatureCode(error.schemaPath, validate);

    if (featureCode) {
      error.featureCode = featureCode;
    }

    error.nameHierarchy = getNameHierarchy(lottie, error.instancePath);

    return error;
  });

  const uniqueErrorMap = enhancedErrors.reduce((errorMap, err) => {
    if (featureErrorsOnly && !err.featureCode) {
      return errorMap;
    }

    const key = `${err.instancePath}:${err.featureCode ?? err.message}`;

    if (!errorMap.has(key)) {
      errorMap.set(key, err);
    }

    return errorMap;
  }, new Map<string, ErrorObject>());

  return [...uniqueErrorMap.values()].sort((e1, e2) => {
    const res = e1.instancePath.localeCompare(e2.instancePath);

    if (res === 0) {
      if (e1.keyword === 'oneOf' && e2.keyword !== 'oneOf') {
        return -1;
      }

      if (e2.keyword === 'oneOf' && e1.keyword !== 'oneOf') {
        return 1;
      }
    }

    return res;
  });
}

function getSchemaPathFeatureCode(
  schemaPath: string,
  validate: ValidateFunction
): string | null {
  const codes = extractPropertiesFromPath(
    validate.schema,
    schemaPath,
    FEATURE_CODE_KEYWORD
  );

  return codes[0] ?? null;
}

function getNameHierarchy(obj: any, instancePath: string): string[] {
  return extractPropertiesFromPath(obj, instancePath, NAME_PROPERTY_PATH);
}

/**
 * Given a '/' separated path, will attempt to traverse the object tree and
 * return the values of the target property from all objects in the tree path
 */
function extractPropertiesFromPath(
  obj: any,
  path: string,
  property: string
): string[] {
  const pathParts = path.split('/');

  const values: string[] = [];
  for (const pathPart of pathParts) {
    if (pathPart === '#' || pathPart === '') {
      continue;
    }

    obj = (obj as any)[pathPart];

    if (!obj) {
      break;
    }

    if (obj[property]) {
      values.push(obj[property]);
    }
  }

  return values;
}

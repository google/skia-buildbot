/** Defines functions and interfaces with working with a shader.
  */

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { Uniform } from '../../../infra-sk/modules/uniform/uniform';
import {
  CanvasKit,
  MallocObj, RuntimeEffect, Shader,
} from '../../build/canvaskit/canvaskit';
import { ScrapBody, ScrapID } from '../json';

export const predefinedUniforms = `uniform float3 iResolution;      // Viewport resolution (pixels)
uniform float  iTime;            // Shader playback time (s)
uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
uniform float3 iImageResolution; // iImage1 and iImage2 resolution (pixels)
uniform shader iImage1;          // An input image (Mandrill).
uniform shader iImage2;          // An input image (Soccer ball).`;

/** How many of the uniforms listed in predefinedUniforms are of type 'shader'? */
export const numPredefinedShaderUniforms = predefinedUniforms.match(/^uniform shader/gm)!.length;

/**
 * Counts the number of uniforms defined in 'predefinedUniforms'. All the
 * remaining uniforms that start with 'i' will be referred to as "user
 * uniforms".
 */
export const numPredefinedUniforms = predefinedUniforms.match(/^uniform/gm)!.length - numPredefinedShaderUniforms;

/**
 * The number of lines prefixed to every shader for predefined uniforms. Needed
 * to properly adjust error line numbers.
 */
export const numPredefinedUniformLines = predefinedUniforms.split('\n').length;

/**
 * Regex that finds lines in shader compiler error messages that mention a line number
 * and makes that line number available as a capture.
 */
export const shaderCompilerErrorRegex = /^error: (\d+)/i;

/** The default shader to fall back to if nothing can be loaded. */
export const defaultShader = `half4 main(float2 fragCoord) {
  return vec4(1, 0, 0, 1);
}`;

/**
 * Called ShaderNode because once we support child shaders this will be just one
 * node in a tree of shaders.
 */
export class ShaderNode {
    // The scrap ID this shader was last saved as.
    private scrapID: string = '';

    // The saved configuration of the shader.
    private body: ScrapBody | null = null;

    // The shader code compiled.
    private effect: RuntimeEffect | null = null;

    private _compileErrorMessage: string = '';

    private _errorLineNumbers: number[] = [];

    private inputImageShaders: Shader[] = [];

    private kit: CanvasKit;

    // Records the code that is currently running.
    private runningCode = defaultShader;

    // The current code in the editor.
    private _shaderCode = defaultShader;

    // These are the uniform values for all the user defined uniforms. They
    // exclude the predefined uniform values.
    private _currentUserUniformValues: number[] = [];

    // Keep a MallocObj around to pass uniforms to the shader to avoid the need to
    // make copies.
    private uniformsMallocObj: MallocObj | null = null;

    constructor(kit: CanvasKit, inputImageShaders: Shader[]) {
      this.kit = kit;
      this.inputImageShaders = inputImageShaders;
    }

    async loadScrap(scrapID: string): Promise<void> {
      this.scrapID = scrapID;
      const resp = await fetch(`/_/load/${scrapID}`, {
        credentials: 'include',
      });
      const json = (await jsonOrThrow(resp)) as ScrapBody;
      this.body = json;
      this._shaderCode = this.body.Body;
      this.currentUserUniformValues = this.body.SKSLMetaData?.Uniforms || [];
      this.runEditedCode();
    }

    async saveScrap(): Promise<string> {
      const body: ScrapBody = {
        Body: this._shaderCode,
        Type: 'sksl',
        SKSLMetaData: {
          Uniforms: this._currentUserUniformValues,
          Children: [],
        },
      };

      // POST the JSON to /_/upload
      const resp = await fetch('/_/save/', {
        credentials: 'include',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
        method: 'POST',
      });
      const json = (await jsonOrThrow(resp)) as ScrapID;
      this.scrapID = json.Hash;
      this.body = body;
      return this.scrapID;
    }

    get shaderCode(): string { return this._shaderCode; }

    set shaderCode(val: string) {
      this._shaderCode = val;
    }

    /** The values that should be used for the used defined uniforms, as opposed to the predefined uniform values. */
    get currentUserUniformValues(): number[] { return this._currentUserUniformValues; }

    set currentUserUniformValues(val: number[]) {
      // Do we need a check that the length matches getUniformFloatCount() - totalPredefinedUniformValues()?
      this._currentUserUniformValues = val;
    }

    /** Only updated on a call to runEditedCode. */
    get compileErrorMessage(): string {
      return this._compileErrorMessage;
    }

    /** Only updated on a call to runEditedCode. */
    get errorLineNumbers(): number[] {
      return this._errorLineNumbers;
    }

    /** Runs the editedCode for this node. */
    runEditedCode(): void {
      this._compileErrorMessage = '';
      this._errorLineNumbers = [];
      this.runningCode = this._shaderCode;
      // eslint-disable-next-line no-unused-expressions
      this.effect?.delete();
      this.effect = this.kit!.RuntimeEffect.Make(`${predefinedUniforms}\n${this.runningCode}`, (err) => {
      // Fix up the line numbers on the error messages, because they are off by
      // the number of lines we prefixed with the predefined uniforms. The regex
      // captures the line number so we can replace it with the correct value.
      // While doing the fix up of the error message we also annotate the
      // corresponding lines in the CodeMirror editor.
        err = err.replace(shaderCompilerErrorRegex, (_match, firstRegexCaptureValue): string => {
          const lineNumber = (+firstRegexCaptureValue - (numPredefinedUniformLines + 1));
          this._errorLineNumbers.push(lineNumber);
          return `error: ${lineNumber.toFixed(0)}`;
        });
        this._compileErrorMessage = err;
      });

      // Copy uniforms into this.uniformsMallocObj, which is kept around to avoid
      // copying overhead in WASM.
      const uniformFloatCount = this.getUniformFloatCount();
      if (!this.uniformsMallocObj) {
        this.uniformsMallocObj = this.kit!.Malloc(Float32Array, uniformFloatCount);
      } else if (this.uniformsMallocObj.length !== uniformFloatCount) {
        this.kit!.Free(this.uniformsMallocObj);
        this.uniformsMallocObj = this.kit!.Malloc(Float32Array, uniformFloatCount);
      }
    }

    /** Returns true if this node or any child nodes need to be run. */
    needsRun(): boolean {
      return (this._shaderCode !== this.runningCode);
    }

    /** Returns true if this node or any child node needs to be saved. */
    needsSave(): boolean {
      return (this._shaderCode !== this.body!.Body) || this.userUniformValuesHaveBeenEdited();
    }

    /** Returns the number of uniforms in the effect. */
    getUniformCount(): number {
      if (!this.effect) {
        return 0;
      }
      return this.effect!.getUniformCount();
    }

    /** Get a description of the uniform at the given index. */
    getUniform(index: number): Uniform {
      // Use object spread operator to clone the SkSLUniform and add a name to make a Uniform.
      return { ...this.effect!.getUniform(index), name: this.effect!.getUniformName(index) };
    }

    /** The total number of floats across all predefined and user uniforms. */
    getUniformFloatCount(): number {
      if (!this.effect) {
        return 0;
      }
      return this.effect!.getUniformFloatCount();
    }

    /**
     * This is really only called once during rAF for the shader that has focus,
     * i.e. that shader that is being displayed on the web UI.
     */
    getShader(predefinedUniformsValues: number[]): Shader | null {
      if (!this.effect) {
        return null;
      }
      const uniformsFloat32Array: Float32Array = this.uniformsMallocObj!.toTypedArray() as Float32Array;

      // Copy in predefined uniforms values.
      predefinedUniformsValues.forEach((val, index) => { uniformsFloat32Array[index] = val; });

      // Copy in our local edited uniform values to the right spots.
      const offsetOfUserDefinedUniforms = this.totalPredefinedUniformValues();
      this.currentUserUniformValues.forEach((val, index) => { uniformsFloat32Array[index + offsetOfUserDefinedUniforms] = val; });

      return this.effect!.makeShaderWithChildren(uniformsFloat32Array, true, this.inputImageShaders);
    }

    /** The number of floats that are defined by predefined uniforms. */
    totalPredefinedUniformValues(): number {
      let ret = 0;
      if (!this.effect) {
        return 0;
      }
      for (let i = 0; i < numPredefinedUniforms; i++) {
        const u = this.effect.getUniform(i);
        ret += u.rows * u.columns;
      }
      return ret;
    }

    private userUniformValuesHaveBeenEdited(): boolean {
      const savedLocalUniformValues = this.body?.SKSLMetaData?.Uniforms || [];
      if (this._currentUserUniformValues.length !== savedLocalUniformValues.length) {
        return true;
      }
      for (let i = 0; i < this._currentUserUniformValues.length; i++) {
        if (this._currentUserUniformValues[i] !== savedLocalUniformValues[i]) {
          return true;
        }
      }
      return false;
    }
}

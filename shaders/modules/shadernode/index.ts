/** Defines functions and interfaces with working with a tree of shaders.
  */

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { Uniform } from '../../../infra-sk/modules/uniform/uniform';
import {
  CanvasKit,
  MallocObj, RuntimeEffect, RuntimeEffectFactory, Shader,
} from '../../build/canvaskit/canvaskit';
import { ScrapBody } from '../json';

export const predefinedUniforms = `uniform float3 iResolution;      // Viewport resolution (pixels)
uniform float  iTime;            // Shader playback time (s)
uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
uniform float3 iImageResolution; // iImage1 and iImage2 resolution (pixels)
uniform shader iImage1;          // An input image (Mandrill).
uniform shader iImage2;          // An input image (Soccer ball).`;

// How many of the uniforms listed in predefinedUniforms are of type 'shader'?
const numPredefinedShaderUniforms = predefinedUniforms.match(/^uniform shader/gm)!.length;

// Counts the number of uniforms defined in 'predefinedUniforms'. All the
// remaining uniforms that start with 'i' will be referred to as "user
// uniforms".
const numPredefinedUniforms = predefinedUniforms.match(/^uniform/gm)!.length - numPredefinedShaderUniforms;

// The number of lines prefixed to every shader for predefined uniforms. Needed
// to properly adjust error line numbers.
const numPredefinedUniformLines = predefinedUniforms.split('\n').length;

// Regex that finds lines in shader compiler error messages that mention a line number
// and makes that line number available as a capture.
const shaderCompilerErrorRegex = /^error: (\d+)/i;

/**
 * predefined uniform values
 * predefined uniform controls
 *
 * root node uniform values
 * children node uniform values
 *
 * root node has a getPredefinedUniforms(): Uniform[].
 * every node has a getUserUniforms(): Uniform[].
 *
 * we read the values out of the predefined uniforms controls on every RaF
 * and pass those down.
 *
 * The user uniforms are only passed to the ShaderNode when a 'change'/'input' event occurs.
 * That way getShader() only needs the predefinedUniformsValues, since the rest are always up to date async from the call to getShader.
 *
 */
const defaultShader = `half4 main(float2 fragCoord) {
  return vec4(1, 0, 0, 1);
}`;

export class ShaderNode {
    // The scrap ID this shader was last saved as.
    private scrapID: string = '';

    // The saved configuration of the shader.
    private body: ScrapBody | null = null;

    // The parent node, if this is a child node, otherwise null.
    private parent: ShaderNode | null = null;

    // The shader code compiled.
    private effect: RuntimeEffect | null = null;

    // Maps child shaders by uniform variable name to children ShaderNode
    // instances.
    private children: ShaderNode[] = [];

    private compileErrorMessage: string = '';

    private errorLineNumbers: number[] = [];

    private inputImageShaders: Shader[] = [];

    private kit: CanvasKit;

    // Records the code that is currently running.
    private runningCode = defaultShader;

    // The current code in the editor.
    private editedCode = defaultShader;

    // These are the uniform values for all the user defined uniforms. They
    // exclude the predefined uniform values.
    private _currentUserUniformValues: number[] = [];

    // Keep a MallocObj around to pass uniforms to the shader to avoid the need to
    // make copies.
    private uniformsMallocObj: MallocObj | null = null;

    constructor(parent: ShaderNode | null, kit: CanvasKit, inputImageShaders: Shader[]) {
      this.parent = parent;
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
      this.shaderCode = this.body.Body;
      this.currentUserUniformValues = this.body.SKSLMetaData?.Uniforms || [];
      // TODO(jcgregorio) When we support child shaders, here is where well need
      // to construct new ShaderNode children for all the children that
      // this.body references.

      this.runEditedCode();
    }

    saveScrap(): void {
      // TBD

      // Remember to update this.scrapID after every save.

      // When we support child shaders we should depth first save all the child
      // shaders that need saving and then save this node if any of the child
      // ids have changed.
    }

    get shaderCode(): string { return this.editedCode; }

    set shaderCode(val: string) {
      this.editedCode = val;
    }

    get currentUserUniformValues(): number[] { return this._currentUserUniformValues; }

    set currentUserUniformValues(val: number[]) { this._currentUserUniformValues = val; }

    runEditedCode(): void {
      this.compileErrorMessage = '';
      this.errorLineNumbers = [];
      this.runningCode = this.editedCode;

      // TODO(jcgregorio) Add support for child shaders by appending the 'uniform shader' variable
      // declarations for each child shader.
      this.effect = this.kit!.RuntimeEffect.Make(`${predefinedUniforms}\n${this.runningCode}`, (err) => {
      // Fix up the line numbers on the error messages, because they are off by
      // the number of lines we prefixed with the predefined uniforms. The regex
      // captures the line number so we can replace it with the correct value.
      // While doing the fix up of the error message we also annotate the
      // corresponding lines in the CodeMirror editor.
        err = err.replace(shaderCompilerErrorRegex, (_match, firstRegexCaptureValue): string => {
          const lineNumber = (+firstRegexCaptureValue - (numPredefinedUniformLines + 1));
          this.errorLineNumbers.push(lineNumber);
          return `error: ${lineNumber.toFixed(0)}`;
        });
        this.compileErrorMessage = err;
      });

      // Copy uniforms into this.uniformsMallocObj, which is kept around to avoid
      // copying overhead in WASM.
      if (!this.uniformsMallocObj) {
        this.uniformsMallocObj = this.kit!.Malloc(Float32Array, this.getUniformFloatCount());
      } else if (this.uniformsMallocObj.length !== this.getUniformFloatCount()) {
        this.kit!.Free(this.uniformsMallocObj);
        this.uniformsMallocObj = this.kit!.Malloc(Float32Array, this.getUniformFloatCount());
      }
    }

    needsRun(): boolean {
      return this.editedCode !== this.runningCode;
    }

    needsSave(): boolean {
      return (this.editedCode !== this.body!.Body) || !this.userUniformValuesHaveBeenEdited() || this.childShaderNamesHaveChanged();
    }

    getUniform(index: number): Uniform {
      return { ...this.effect!.getUniform(index), name: this.effect!.getUniformName(index) };
    }

    getUniformFloatCount(): number {
      return this.effect!.getUniformFloatCount();
    }

    // This is really only called once during rAF for the shader that has focus,
    // i.e. is being displayed on the web UI. It calls getShader() on all its
    // children to build the final shader.
    getShader(predefinedUniformsValues: number[]): Shader {
      const uniformsFloat32Array: Float32Array = this.uniformsMallocObj!.toTypedArray() as Float32Array;

      // Copy in predefined uniforms values.
      predefinedUniformsValues.forEach((val, index) => { uniformsFloat32Array[index] = val; });

      // Copy in our local edited uniform values to the right spots.
      const offsetOfUserDefinedUniforms = this.totalPredefinedUniformValues();
      this.currentUserUniformValues.forEach((val, index) => { uniformsFloat32Array[index + offsetOfUserDefinedUniforms] = val; });

      // Get all the child shaders.
      const childShaders = this.children.map((childShaderNode) => childShaderNode.getShader(predefinedUniformsValues));
      return this.effect!.makeShaderWithChildren(uniformsFloat32Array, true, this.inputImageShaders.concat(childShaders));
    }

    private childShaderNamesHaveChanged(): boolean {
      const expectedChildIDs = (this.body?.SKSLMetaData?.Children || []).map((childShader) => childShader.ScrapHashOrName);
      const actualChildIDs = this.children.map((child) => child.scrapID);
      if (expectedChildIDs.length !== actualChildIDs.length) {
        return false;
      }
      return expectedChildIDs.every((value, index) => value === actualChildIDs[index]);
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

    private totalPredefinedUniformValues(): number {
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
}

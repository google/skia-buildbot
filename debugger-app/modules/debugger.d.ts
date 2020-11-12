// Contains the type definitions for all the emscripten bound methods from the wasm debugger.
// These are defined in //skia/experimental/wasm-skp-debugger/debugger_bindings.cpp

export interface DebuggerInitOptions {
    locateFile: (path: string) => string;
};
export interface Debugger {
  // defined in wasm-skp-debugger/helper.js
  SkpFilePlayer(ab: ArrayBuffer): SkpFilePlayerResult;
  MakeWebGLCanvasSurface(canvas: HTMLCanvasElement): SkSurface;
  MakeSWCanvasSurface(canvas: HTMLCanvasElement): SkSurface;
};
// An object containing either the successfully loaded file player or an error.
export interface SkpFilePlayerResult {
  readonly error: string;
  readonly player: SkpDebugPlayer;
};
export interface SkpDebugPlayer {
  changeFrame(index: number): void;
  deleteCommand(index: number): void;
  draw(surface: SkSurface): void;
  drawTo(surface: SkSurface, index: number): void;
  fileVersion(): number;
  findCommandByPixel(surface: SkSurface, x: number, y: number, upperBound: number): number;
  getBounds(): SkIRect;
  getFrameCount(): number;
  getImageResource(index: number): string;
  getImageCount(): number;
  getImageInfo(index: number): SimpleImageInfo;
  getLayerSummariesJs(): LayerSummary[];
  getSize(): number;
  imageUseInfoForFrame(frame: number): string;
  jsonCommandList(surface: SkSurface): string;
  lastCommandInfo(): string;
  loadSkp(ptr: number, len: number): string;
  setClipVizColor(color: Color): void;
  setCommandVisibility(index: number, visible: boolean): void;
  setGpuOpBounds(visible: boolean): void;
  setInspectedLayer(nodeId: number): void;
  setOriginVisible(visible: boolean): void;
  setOverdrawVis(visible: boolean): void;
  setAndroidClipViz(visible: boolean): void;
  TRANSPARENT: number;
};
export interface SkSurface {
  dispose(): void;
  flush(): void;
  clear(color: Color): void;
};
export interface SimpleImageInfo {

};
export interface SkIRect {
  fLeft: number;
  fTop: number;
  fRight: number;
  fBottom: number;
}
export interface Color {

};
export type Matrix3x3 = [
  [number, number, number],
  [number, number, number],
  [number, number, number],
];
export type Matrix4x4 = [
  [number, number, number, number],
  [number, number, number, number],
  [number, number, number, number],
  [number, number, number, number],
];
// return type of lastCommandInfo after json.parse
export interface MatrixClipInfo {
  ClipRect: [number, number, number, number],
  ViewMatrix: Matrix3x3 | Matrix4x4,
};

export interface SkpJsonGpuOp {
  Name: string,
  ClientID: number,
  OpsTaskID: number,
  ChildID: number,
  // TODO(nifong): bounds, stack
};
export interface SkpJsonAuditTrail {
  Ops: SkpJsonGpuOp[],
};
export interface SkpJsonCommand {
  command: string // name
  shortDesc?: string // short description
  layerNodeId?: number,
  imageIndex?: number,
  key?: string, // text that goes along with a DrawAnnotation command
  auditTrail: SkpJsonAuditTrail,
};
// Return type of SkpDebugPlayer.jsonCommandList() after json.parse
export interface SkpJsonCommandList {
  commands: SkpJsonCommand[],
};
// Info about layer events relevant to a particular layer and frame
// struct from LayerManager.h
export interface LayerSummary {
  nodeId: number,
  frameOfLastUpdate: number,
  fullRedraw: boolean,
  layerWidth: number,
  layerHeight: number
}
// Contains the type definitions for all the emscripten bound methods from the wasm debugger.
// These are defined in //skia/experimental/wasm-skp-debugger/debugger_bindings.cpp
import type {
  CanvasKit,
  Surface
} from '../wasm_libs/types/canvaskit';

export interface Debugger extends CanvasKit {
  SkpFilePlayer(ab: ArrayBuffer): SkpFilePlayerResult;
  MinVersion(): number;
}
// An object containing either the successfully loaded file player or an error.
export interface SkpFilePlayerResult {
  readonly error: string;
  readonly player: SkpDebugPlayer;
}
export interface SkpDebugPlayer {
  changeFrame(index: number): void;
  deleteCommand(index: number): void;
  draw(surface: Surface): void;
  drawTo(surface: Surface, index: number): void;
  findCommandByPixel(surface: Surface, x: number, y: number, upperBound: number): number;
  getBounds(): SkIRect;
  getBoundsForFrame(frame: number): SkIRect;
  getFrameCount(): number;
  getImageResource(index: number): string;
  getImageCount(): number;
  getImageInfo(index: number): ImageInfoNoColorspace;
  getLayerKeys(): LayerKey[]
  getLayerSummariesJs(): LayerSummary[];
  getSize(): number;
  imageUseInfo(frame: number, nodeid: number): ImageUseMap;
  jsonCommandList(surface: Surface): string;
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
}
export interface ImageInfoNoColorspace {
  width: number,
  height: number,
  colorType: number,
  alphaType: number,
}
export interface SkIRect {
  fLeft: number;
  fTop: number;
  fRight: number;
  fBottom: number;
}
export type Color = number;
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
}

export interface SkpJsonGpuOp {
  Name: string,
  ClientID: number,
  OpsTaskID: number,
  ChildID: number,
  // TODO(nifong): bounds, stack
}
export interface SkpJsonAuditTrail {
  Ops: SkpJsonGpuOp[],
}
export interface SkpJsonCommand {
  command: string // name
  shortDesc?: string // short description
  layerNodeId?: number,
  imageIndex?: number,
  key?: string, // text that goes along with a DrawAnnotation command
  auditTrail: SkpJsonAuditTrail,
}
// Return type of SkpDebugPlayer.jsonCommandList() after json.parse
export interface SkpJsonCommandList {
  commands: SkpJsonCommand[],
}
// Indentifier of a layer update evenet
export interface LayerKey {
  frame: number,
  nodeId: number,
}
// Info about layer events relevant to a particular layer and frame
// struct from LayerManager.h
export interface LayerSummary {
  nodeId: number,
  frameOfLastUpdate: number,
  fullRedraw: boolean,
  layerWidth: number,
  layerHeight: number,
}
// return type of imageUseInfoForFrame
// Keys are image ids, values are lists of command indices
export type ImageUseMap = Map<string, number[]>

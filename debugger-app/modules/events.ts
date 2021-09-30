// An entry of the command histogram obtained by totalling up
//  occurrences in the range filtered command list
export interface HistogramEntry {
    // name of a command (original CamelCase)
    name: string,
    // number of occurrences in the current frame (or the whole file for a single-frame SKP)
    countInFrame: number,
    // number of occurrences in the current range filter
    countInRange: number,
}

// An event signaling the histogram-sk element should update itself
export const HistogramUpdateEvent = 'histogram-update';
export interface HistogramUpdateEventDetail {
    // A newly computed histogram that needs to be displayed by histogram-sk
    hist?: HistogramEntry[];
    // Whether the command is include by the filter
    included?: Set<string>;
}

// An event to trigger the inspector for a given layer update.
// A layer update is fully specified by the node id and a frame on which an
// update to it occurred.
export const InspectLayerEvent = 'inspect-layer';
export const JumpInspectLayerEvent = 'jump-inspect-layer';
export interface InspectLayerEventDetail {
    id: number;
    frame: number;
}

// Event issued when the user wants to jump to a command, specifying the unfiltered index
// in the detail.
export const JumpCommandEvent = 'jump-command';
export interface JumpCommandEventDetail {
    unfilteredIndex: number;
}

export type PlayMode = 'play' | 'pause';

export const ModeChangedManuallyEvent = 'mode-changed-manually';
export interface ModeChangedManuallyEventDetail {
    readonly mode: PlayMode;
}

export const MoveCommandPositionEvent = 'move-command-position';
export interface MoveCommandPositionEventDetail {
    // the index of a command in the frame to which the wasm view should move.
    position: number,
    // true if we're currently paused
    paused: boolean,
}

export type Point = [number, number];

export const MoveCursorEvent = 'move-cursor';
export const RenderCursorEvent = 'render-cursor';
// This event detail is used for both move-cursor and render-cursor
export interface CursorEventDetail {
    // the position of the cursor.
    position: Point,
    // If true, indicates only the data under the cursor has changed.
    // since some consumers don't need to update in this case.
    onlyData: boolean,
}

export const MoveFrameEvent = 'move-frame';
export interface MoveFrameEventDetail {
    frame: number,
}

export const MoveToEvent = 'moveto';
export interface MoveToEventDetail {
    readonly item: number
}

export const NextItemEvent = 'next-item';
export interface NextItemEventDetail {
    item: number;
}

// Event issued when the user clicks on an 'Image', indicating to jump to the image with the
// id in the detail.
export const SelectImageEvent = 'select-image';
export interface SelectImageEventDetail {
    id: number;
}
export type BackgroundStyle = 'dark-checkerboard' | 'light-checkerboard';

export const ToggleBackgroundEvent = 'light-dark';
export interface ToggleBackgroundEventDetail {
    mode: BackgroundStyle;
}

export const ToggleCommandInclusionEvent = 'toggle-command-inclusion';
export interface ToggleCommandInclusionEventDetail {
    // the name of a command to toggle in the filter
    // Clicking rows of the histogram is an alternate way to add or remove command names
    // from the command filter. The filter is managed by commands-sk
    name: string,
}

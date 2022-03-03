/** Test module to check whether sourcemaps are generated. Its contents do not matter. */

function makeMsg(name: string): string {
    return `Hello, ${name}!`;
}

export const msg = makeMsg('World');

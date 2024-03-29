/* eslint-disable no-console */
// gentheme generates the token values for a CSS theme.
//
//  bazelisk run //infra-sk/modules/gentheme/cmd:gentheme -- 005db7 006e1c $HOME/theme.scss
//
import fs from 'fs';
import { gentheme } from '../../gentheme';

// The 'node' exe is also passed as the first arg, so test for N + 1 args.
if (process.argv.length !== 5) {
  console.error(
    `Usage: gentheme <primary-color-as-hex> <secondary-color-as-hex> <filename>
  Generates a theme token file for the given colors.`
  );
  process.exit(1);
}

const fileContents = `// DO NOT EDIT
//
// This file is generated by //infra-sk/modules/gentheme/cmd:gentheme.
//
// Primary seed: #${process.argv[2]}
// Secondary seed: #${process.argv[3]}

${gentheme(`#${process.argv[2]}`, `#${process.argv[3]}`)}
`;

try {
  fs.writeFileSync(process.argv[4], fileContents);
} catch (err) {
  console.error(err);
  process.exit(1);
}

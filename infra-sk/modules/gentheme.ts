// gentheme generates the token values for a CSS theme.
//
import {
  argbFromHex,
  hexFromArgb,
  MaterialDynamicColors,
  Hct,
  SchemeTonalSpot,
  DynamicScheme,
  TonalPalette,
  sanitizeDegreesDouble,
} from '@material/material-color-utilities';

// Create our own Scheme.
//
// This is essentially a copy of the Material Color Utilities SchemaTonalSpot,
// which is the default Material You theme on Android 12 and 13, but with the
// addition of a secondary color that also has a Chroma of 36, just like the
// primary color.
class MySchemeTonalSpot extends DynamicScheme {
  constructor(
    sourceColorHct: Hct,
    secondaryColorHct: Hct,
    isDark: boolean,
    contrastLevel: number
  ) {
    super({
      sourceColorArgb: sourceColorHct.toInt(),
      variant: 2, // Variant.TONAL_SPOT,
      contrastLevel,
      isDark,
      primaryPalette: TonalPalette.fromHueAndChroma(sourceColorHct.hue, 36.0),
      secondaryPalette: TonalPalette.fromHueAndChroma(
        secondaryColorHct.hue,
        36.0
      ),
      tertiaryPalette: TonalPalette.fromHueAndChroma(
        sanitizeDegreesDouble(sourceColorHct.hue + 60.0),
        24.0
      ),
      neutralPalette: TonalPalette.fromHueAndChroma(sourceColorHct.hue, 6.0),
      neutralVariantPalette: TonalPalette.fromHueAndChroma(
        sourceColorHct.hue,
        8.0
      ),
    });
  }
}

/** Generates a theme as a CSS string that defines CSS variables with all the
 * colors used in the Skia Infra theme starting from two colors that are passed
 * in as hex string values, e.g. "#009900".
 *
 * See //infra-sk/theme.scss for how the tokens are used.
 */
export const gentheme = (primary: string, secondary: string): string => {
  const primarySeed = argbFromHex(primary);
  const secondarySeed = argbFromHex(secondary);

  const darkScheme = new MySchemeTonalSpot(
    Hct.fromInt(primarySeed),
    Hct.fromInt(secondarySeed),
    true,
    0.4
  );
  const lightScheme = new MySchemeTonalSpot(
    Hct.fromInt(primarySeed),
    Hct.fromInt(secondarySeed),
    false,
    0.4
  );

  // prettier-ignore
  const tokensFromDynamicScheme = (selectors: string, scheme: SchemeTonalSpot) => `${selectors} {
  --background:              ${hexFromArgb(MaterialDynamicColors.background.getArgb(scheme))};
  --disabled:                ${hexFromArgb(MaterialDynamicColors.surfaceVariant.getArgb(scheme))};
  --error:                   ${hexFromArgb(MaterialDynamicColors.error.getArgb(scheme))};
  --error-container:         ${hexFromArgb(MaterialDynamicColors.errorContainer.getArgb(scheme))};
  --on-error-container:      ${hexFromArgb(MaterialDynamicColors.onErrorContainer.getArgb(scheme))};
  --on-background:           ${hexFromArgb(MaterialDynamicColors.onBackground.getArgb(scheme))};
  --on-disabled:             ${hexFromArgb(MaterialDynamicColors.onSurfaceVariant.getArgb(scheme))};
  --on-error:                ${hexFromArgb(MaterialDynamicColors.onError.getArgb(scheme))};
  --on-primary:              ${hexFromArgb(MaterialDynamicColors.onPrimary.getArgb(scheme))};
  --on-secondary:            ${hexFromArgb(MaterialDynamicColors.onSecondary.getArgb(scheme))};
  --on-surface:              ${hexFromArgb(MaterialDynamicColors.onSurface.getArgb(scheme))};
  --primary:                 ${hexFromArgb(MaterialDynamicColors.primary.getArgb(scheme))};
  --secondary:               ${hexFromArgb(MaterialDynamicColors.secondary.getArgb(scheme))};
  --surface:                 ${hexFromArgb(MaterialDynamicColors.surfaceContainer.getArgb(scheme))};
  --surface-1dp:             ${hexFromArgb(MaterialDynamicColors.surfaceContainerHigh.getArgb(scheme))};
  --surface-2dp:             ${hexFromArgb(MaterialDynamicColors.surfaceContainerHighest.getArgb(scheme))};
  --outline:                 ${hexFromArgb(MaterialDynamicColors.outline.getArgb(scheme))};
  --primary-highlight:       ${hexFromArgb(MaterialDynamicColors.primaryContainer.getArgb(scheme))};
  --on-primary-highlight:    ${hexFromArgb(MaterialDynamicColors.onPrimaryContainer.getArgb(scheme))};
  --secondary-highlight:     ${hexFromArgb(MaterialDynamicColors.secondaryContainer.getArgb(scheme))};
  --on-hightlight:           ${hexFromArgb(MaterialDynamicColors.onPrimaryContainer.getArgb(scheme))};
  --on-secondary-highlight:  ${hexFromArgb(MaterialDynamicColors.onSecondaryContainer.getArgb(scheme))};
  --primary-variant:         ${hexFromArgb(MaterialDynamicColors.primaryContainer.getArgb(scheme))};
  --on-primary-variant:      ${hexFromArgb(MaterialDynamicColors.onPrimaryContainer.getArgb(scheme))};
  --surface-1dp:             ${hexFromArgb(MaterialDynamicColors.surfaceContainerHigh.getArgb(scheme))};
  --surface-2dp:             ${hexFromArgb(MaterialDynamicColors.surfaceContainerHighest.getArgb(scheme))};
}`;

  // Format as CSS and not SCSS so that the output can be used directly in the
  // theme-chooser-sk demo page.
  return `/* prettier-ignore */
${tokensFromDynamicScheme('.body-sk', lightScheme)}

/* prettier-ignore */
${tokensFromDynamicScheme('.body-sk.darkmode,.body-sk .darkmode', darkScheme)}`;
};

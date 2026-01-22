/**
 * Pressing Button Assets
 *
 * Local imports for pressing button images with normal and pressed states.
 * These are bundled with the app via Vite's asset handling.
 */

// Long buttons - normal state
import buttonLongBeige from '../../assets/images/pressing_buttons/buttonLong_beige.png'
import buttonLongBlue from '../../assets/images/pressing_buttons/buttonLong_blue.png'
import buttonLongBrown from '../../assets/images/pressing_buttons/buttonLong_brown.png'
import buttonLongGrey from '../../assets/images/pressing_buttons/buttonLong_grey.png'

// Long buttons - pressed state
import buttonLongBeigePressed from '../../assets/images/pressing_buttons/buttonLong_beige_pressed.png'
import buttonLongBluePressed from '../../assets/images/pressing_buttons/buttonLong_blue_pressed.png'
import buttonLongBrownPressed from '../../assets/images/pressing_buttons/buttonLong_brown_pressed.png'
import buttonLongGreyPressed from '../../assets/images/pressing_buttons/buttonLong_grey_pressed.png'

// Square buttons - normal state
import buttonSquareBeige from '../../assets/images/pressing_buttons/buttonSquare_beige.png'
import buttonSquareBlue from '../../assets/images/pressing_buttons/buttonSquare_blue.png'
import buttonSquareBrown from '../../assets/images/pressing_buttons/buttonSquare_brown.png'
import buttonSquareGrey from '../../assets/images/pressing_buttons/buttonSquare_grey.png'

// Square buttons - pressed state
import buttonSquareBeigePressed from '../../assets/images/pressing_buttons/buttonSquare_beige_pressed.png'
import buttonSquareBluePressed from '../../assets/images/pressing_buttons/buttonSquare_blue_pressed.png'
import buttonSquareBrownPressed from '../../assets/images/pressing_buttons/buttonSquare_brown_pressed.png'
import buttonSquareGreyPressed from '../../assets/images/pressing_buttons/buttonSquare_grey_pressed.png'

export type ButtonColor = 'beige' | 'blue' | 'brown' | 'grey'
export type ButtonShape = 'long' | 'square'

export interface ButtonImages {
  normal: string
  pressed: string
}

// Long button images by color
export const longButtons: Record<ButtonColor, ButtonImages> = {
  beige: { normal: buttonLongBeige, pressed: buttonLongBeigePressed },
  blue: { normal: buttonLongBlue, pressed: buttonLongBluePressed },
  brown: { normal: buttonLongBrown, pressed: buttonLongBrownPressed },
  grey: { normal: buttonLongGrey, pressed: buttonLongGreyPressed },
}

// Square button images by color
export const squareButtons: Record<ButtonColor, ButtonImages> = {
  beige: { normal: buttonSquareBeige, pressed: buttonSquareBeigePressed },
  blue: { normal: buttonSquareBlue, pressed: buttonSquareBluePressed },
  brown: { normal: buttonSquareBrown, pressed: buttonSquareBrownPressed },
  grey: { normal: buttonSquareGrey, pressed: buttonSquareGreyPressed },
}

/**
 * Get button images for a specific color and shape.
 */
export function getButtonImages(color: ButtonColor, shape: ButtonShape): ButtonImages {
  return shape === 'long' ? longButtons[color] : squareButtons[color]
}

// Map game button variants to pressing button colors
export type GameButtonVariant = 'primary' | 'secondary' | 'neutral' | 'danger'

export const variantToColor: Record<GameButtonVariant, ButtonColor> = {
  primary: 'brown',
  secondary: 'blue',
  neutral: 'beige',
  danger: 'grey',
}

/**
 * Get button images for a game button variant.
 * Uses long buttons by default for standard game buttons.
 */
export function getVariantButtonImages(
  variant: GameButtonVariant,
  shape: ButtonShape = 'long'
): ButtonImages {
  const color = variantToColor[variant]
  return getButtonImages(color, shape)
}

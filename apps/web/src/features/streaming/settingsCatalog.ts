import light from '../../assets/backgrounds/light-studio-thumb.webp?url'
import dark from '../../assets/backgrounds/dark-lounge-thumb.webp?url'
import natural from '../../assets/backgrounds/natural-space-thumb.webp?url'

import lightImage from '../../assets/backgrounds/light-studio.webp?url'
import darkImage from '../../assets/backgrounds/dark-lounge.webp?url'
import naturalImage from '../../assets/backgrounds/natural-space.webp?url'

// Public background images only. Character packages and materials stay private.
export const backgrounds = [
  { id: 'light-studio', name: 'Light Studio', thumbnail: light, image: lightImage },
  { id: 'dark-lounge', name: 'Dark Lounge', thumbnail: dark, image: darkImage },
  { id: 'natural-space', name: 'Natural Space', thumbnail: natural, image: naturalImage },
] as const

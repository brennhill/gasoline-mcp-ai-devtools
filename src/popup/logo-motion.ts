/**
 * Purpose: Applies shared STRUM logo idle/hover motion behavior inside extension popup surfaces.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

/**
 * Initialize popup logo motion.
 * The shared icon asset carries the slow idle movement; hover escalates to the stronger strum asset.
 */
export function initPopupLogoMotion(): void {
  const logo = document.querySelector('.logo') as HTMLImageElement | null
  if (!logo) return

  const idleSrc = chrome.runtime.getURL('icons/icon.svg')
  const hoverSrc = chrome.runtime.getURL('icons/logo-animated.svg')

  logo.src = idleSrc
  logo.addEventListener('mouseenter', () => {
    logo.src = hoverSrc
  })
  logo.addEventListener('mouseleave', () => {
    logo.src = idleSrc
  })
}

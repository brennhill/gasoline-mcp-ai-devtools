// @ts-nocheck
/**
 * @fileoverview Options page logic for Gasoline extension
 */

const DEFAULT_SERVER_URL = 'http://localhost:7890'

// Load saved options
function loadOptions() {
  chrome.storage.local.get(['serverUrl', 'domainFilters', 'screenshotOnError', 'sourceMapEnabled'], (result) => {
    // Set server URL
    document.getElementById('server-url-input').value = result.serverUrl || DEFAULT_SERVER_URL

    const filters = result.domainFilters || []
    document.getElementById('domain-filters').value = filters.join('\n')

    // Set screenshot toggle state
    const screenshotToggle = document.getElementById('screenshot-toggle')
    if (result.screenshotOnError) {
      screenshotToggle.classList.add('active')
    }

    // Set source map toggle state
    const sourcemapToggle = document.getElementById('sourcemap-toggle')
    if (result.sourceMapEnabled) {
      sourcemapToggle.classList.add('active')
    }
  })
}

// Save options
function saveOptions() {
  const serverUrl = document.getElementById('server-url-input').value.trim() || DEFAULT_SERVER_URL

  const textarea = document.getElementById('domain-filters')
  const filters = textarea.value
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0)

  const screenshotOnError = document.getElementById('screenshot-toggle').classList.contains('active')
  const sourceMapEnabled = document.getElementById('sourcemap-toggle').classList.contains('active')

  chrome.storage.local.set({ serverUrl, domainFilters: filters, screenshotOnError, sourceMapEnabled }, () => {
    // Show saved message
    const message = document.getElementById('saved-message')
    message.classList.add('show')

    // Notify background
    chrome.runtime.sendMessage({ type: 'setServerUrl', url: serverUrl })
    chrome.runtime.sendMessage({ type: 'setDomainFilters', filters })
    chrome.runtime.sendMessage({ type: 'setScreenshotOnError', enabled: screenshotOnError })
    chrome.runtime.sendMessage({ type: 'setSourceMapEnabled', enabled: sourceMapEnabled })

    // Hide message after 2 seconds
    setTimeout(() => {
      message.classList.remove('show')
    }, 2000)
  })
}

// Toggle screenshot setting
function toggleScreenshot() {
  const toggle = document.getElementById('screenshot-toggle')
  toggle.classList.toggle('active')
}

// Toggle source map setting
function toggleSourceMap() {
  const toggle = document.getElementById('sourcemap-toggle')
  toggle.classList.toggle('active')
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  loadOptions()
  document.getElementById('save-btn').addEventListener('click', saveOptions)
  document.getElementById('screenshot-toggle').addEventListener('click', toggleScreenshot)
  document.getElementById('sourcemap-toggle').addEventListener('click', toggleSourceMap)
})

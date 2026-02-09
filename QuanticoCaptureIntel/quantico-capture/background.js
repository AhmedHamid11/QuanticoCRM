// QuanticoCRM Capture - Background Service Worker

// Listen for messages from popup
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'captureScreenshot') {
    captureCurrentTab()
      .then(screenshot => sendResponse({ success: true, screenshot }))
      .catch(error => sendResponse({ success: false, error: error.message }));
    return true; // Keep channel open for async response
  }

  if (request.action === 'getTabInfo') {
    getCurrentTabInfo()
      .then(tabInfo => sendResponse({ success: true, tabInfo }))
      .catch(error => sendResponse({ success: false, error: error.message }));
    return true;
  }

  return false;
});

// Capture screenshot of current visible tab
async function captureCurrentTab() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab) {
      throw new Error('No active tab found');
    }

    const screenshot = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: 'png'
    });

    return screenshot;
  } catch (error) {
    console.error('Capture error:', error);
    throw error;
  }
}

// Get current tab URL and title
async function getCurrentTabInfo() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab) {
      throw new Error('No active tab found');
    }

    return {
      url: tab.url || '',
      title: tab.title || '',
      domain: extractDomain(tab.url),
      favicon: tab.favIconUrl || ''
    };
  } catch (error) {
    console.error('Tab info error:', error);
    throw error;
  }
}

// Extract domain from URL
function extractDomain(url) {
  try {
    if (!url) return 'unknown';
    const urlObj = new URL(url);
    return urlObj.hostname.replace('www.', '');
  } catch {
    return 'unknown';
  }
}

console.log('QuanticoCRM Capture service worker loaded');

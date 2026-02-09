// QuanticoCRM Capture - Popup Script

// State
let currentScreenshot = null;
let currentTabInfo = null;
let selectedContact = null;
let contacts = []; // Cached contacts
let searchTimeout = null;

// DOM Elements
const elements = {
  // Settings
  settingsPanel: document.getElementById('settingsPanel'),
  settingsToggle: document.getElementById('settingsToggle'),
  webhookUrl: document.getElementById('webhookUrl'),
  crmApiUrl: document.getElementById('crmApiUrl'),
  apiKey: document.getElementById('apiKey'),
  saveSettings: document.getElementById('saveSettings'),
  
  // Screenshot
  screenshotPlaceholder: document.getElementById('screenshotPlaceholder'),
  screenshotPreview: document.getElementById('screenshotPreview'),
  
  // Page Info
  pageInfo: document.getElementById('pageInfo'),
  pageFavicon: document.getElementById('pageFavicon'),
  pageDomain: document.getElementById('pageDomain'),
  pageTitle: document.getElementById('pageTitle'),
  
  // Contact
  contactSearch: document.getElementById('contactSearch'),
  contactResults: document.getElementById('contactResults'),
  selectedContact: document.getElementById('selectedContact'),
  selectedAvatar: document.getElementById('selectedAvatar'),
  selectedName: document.getElementById('selectedName'),
  selectedEmail: document.getElementById('selectedEmail'),
  clearContact: document.getElementById('clearContact'),
  
  // Note
  taskNote: document.getElementById('taskNote'),
  
  // Actions
  captureBtn: document.getElementById('captureBtn'),
  sendBtn: document.getElementById('sendBtn'),
  
  // Status
  statusMessage: document.getElementById('statusMessage')
};

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  setupEventListeners();
  
  // Auto-capture on open (optional - remove if not desired)
  // captureScreenshot();
});

// Event Listeners
function setupEventListeners() {
  // Settings
  elements.settingsToggle.addEventListener('click', toggleSettings);
  elements.saveSettings.addEventListener('click', saveSettings);
  
  // Capture
  elements.captureBtn.addEventListener('click', captureScreenshot);
  
  // Contact Search
  elements.contactSearch.addEventListener('input', handleContactSearch);
  elements.contactSearch.addEventListener('focus', () => {
    if (contacts.length > 0) {
      showContactResults(contacts);
    }
  });
  
  // Clear Contact
  elements.clearContact.addEventListener('click', clearSelectedContact);
  
  // Send
  elements.sendBtn.addEventListener('click', sendToWebhook);
  
  // Close results on outside click
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.contact-section')) {
      elements.contactResults.classList.add('hidden');
    }
  });
}

// Settings Management
function toggleSettings() {
  elements.settingsPanel.classList.toggle('hidden');
}

async function loadSettings() {
  const settings = await chrome.storage.sync.get(['webhookUrl', 'crmApiUrl', 'apiKey']);
  if (settings.webhookUrl) elements.webhookUrl.value = settings.webhookUrl;
  if (settings.crmApiUrl) elements.crmApiUrl.value = settings.crmApiUrl;
  if (settings.apiKey) elements.apiKey.value = settings.apiKey;
}

async function saveSettings() {
  const settings = {
    webhookUrl: elements.webhookUrl.value.trim(),
    crmApiUrl: elements.crmApiUrl.value.trim(),
    apiKey: elements.apiKey.value.trim()
  };
  
  await chrome.storage.sync.set(settings);
  showStatus('Settings saved!', 'success');
  elements.settingsPanel.classList.add('hidden');
  
  // Fetch contacts with new settings
  if (settings.crmApiUrl && settings.apiKey) {
    fetchContacts();
  }
}

// Screenshot Capture
async function captureScreenshot() {
  try {
    showStatus('Capturing...', 'loading');
    
    // Get tab info
    const tabResponse = await chrome.runtime.sendMessage({ action: 'getTabInfo' });
    if (!tabResponse.success) {
      throw new Error(tabResponse.error);
    }
    currentTabInfo = tabResponse.tabInfo;
    
    // Capture screenshot
    const screenshotResponse = await chrome.runtime.sendMessage({ action: 'captureScreenshot' });
    if (!screenshotResponse.success) {
      throw new Error(screenshotResponse.error);
    }
    currentScreenshot = screenshotResponse.screenshot;
    
    // Update UI
    elements.screenshotPlaceholder.style.display = 'none';
    elements.screenshotPreview.src = currentScreenshot;
    elements.screenshotPreview.classList.remove('hidden');
    
    // Show page info
    if (currentTabInfo.favicon) {
      elements.pageFavicon.src = currentTabInfo.favicon;
      elements.pageFavicon.style.display = 'block';
    } else {
      elements.pageFavicon.style.display = 'none';
    }
    elements.pageDomain.textContent = currentTabInfo.domain;
    elements.pageTitle.textContent = currentTabInfo.title;
    elements.pageInfo.classList.remove('hidden');
    
    updateSendButton();
    hideStatus();
    
  } catch (error) {
    console.error('Capture error:', error);
    showStatus(`Capture failed: ${error.message}`, 'error');
  }
}

// Contact Search
async function handleContactSearch(e) {
  const query = e.target.value.trim();
  
  // Debounce
  if (searchTimeout) clearTimeout(searchTimeout);
  
  if (query.length < 2) {
    elements.contactResults.classList.add('hidden');
    return;
  }
  
  searchTimeout = setTimeout(() => {
    searchContacts(query);
  }, 300);
}

async function searchContacts(query) {
  const settings = await chrome.storage.sync.get(['crmApiUrl', 'apiKey']);
  
  if (!settings.crmApiUrl || !settings.apiKey) {
    showStatus('Configure CRM settings first', 'error');
    toggleSettings();
    return;
  }
  
  try {
    // Call QuanticoCRM API
    const response = await fetch(`${settings.crmApiUrl}/contacts?search=${encodeURIComponent(query)}&pageSize=10`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${settings.apiKey}`,
        'Content-Type': 'application/json'
      }
    });
    
    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }
    
    const data = await response.json();
    contacts = data.data || data.list || [];
    
    showContactResults(contacts);
    
  } catch (error) {
    console.error('Contact search error:', error);
    
    // For demo/testing: show mock results
    if (error.message.includes('Failed to fetch') || error.message.includes('NetworkError')) {
      // Mock data for testing without API
      contacts = getMockContacts(query);
      showContactResults(contacts);
    } else {
      showStatus(`Search failed: ${error.message}`, 'error');
    }
  }
}

async function fetchContacts() {
  const settings = await chrome.storage.sync.get(['crmApiUrl', 'apiKey']);
  
  if (!settings.crmApiUrl || !settings.apiKey) return;
  
  try {
    const response = await fetch(`${settings.crmApiUrl}/contacts?pageSize=50&sortBy=first_name`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${settings.apiKey}`,
        'Content-Type': 'application/json'
      }
    });
    
    if (response.ok) {
      const data = await response.json();
      contacts = data.data || data.list || [];
    }
  } catch (error) {
    console.error('Failed to fetch contacts:', error);
  }
}

// Mock data for testing
function getMockContacts(query) {
  const mockContacts = [
    { id: '1', name: 'John Smith', emailAddress: 'john.smith@example.com' },
    { id: '2', name: 'Sarah Johnson', emailAddress: 'sarah.j@company.com' },
    { id: '3', name: 'Mike Williams', emailAddress: 'mike.w@business.net' },
    { id: '4', name: 'Emily Brown', emailAddress: 'emily.brown@corp.io' },
    { id: '5', name: 'David Lee', emailAddress: 'david.lee@startup.co' }
  ];
  
  return mockContacts.filter(c => 
    c.name.toLowerCase().includes(query.toLowerCase()) ||
    c.emailAddress.toLowerCase().includes(query.toLowerCase())
  );
}

function showContactResults(contactList) {
  if (contactList.length === 0) {
    elements.contactResults.innerHTML = '<div class="no-results">No contacts found</div>';
  } else {
    elements.contactResults.innerHTML = contactList.map(contact => {
      // Support both FastCRM format (firstName/lastName) and legacy format (name)
      const name = contact.name || `${contact.first_name || contact.firstName || ''} ${contact.last_name || contact.lastName || ''}`.trim();
      const email = contact.email || contact.emailAddress || '';
      return `
      <div class="contact-card" data-id="${contact.id}" data-name="${escapeHtml(name)}" data-email="${escapeHtml(email)}">
        <div class="contact-avatar">${getInitials(name)}</div>
        <div class="contact-info">
          <div class="contact-name">${escapeHtml(name)}</div>
          <div class="contact-email">${escapeHtml(email || 'No email')}</div>
        </div>
      </div>
    `}).join('');
    
    // Add click handlers
    elements.contactResults.querySelectorAll('.contact-card').forEach(card => {
      card.addEventListener('click', () => selectContact(card.dataset));
    });
  }
  
  elements.contactResults.classList.remove('hidden');
}

function selectContact(contactData) {
  selectedContact = {
    id: contactData.id,
    name: contactData.name,
    email: contactData.email
  };
  
  // Update UI
  elements.selectedAvatar.textContent = getInitials(selectedContact.name);
  elements.selectedName.textContent = selectedContact.name;
  elements.selectedEmail.textContent = selectedContact.email || 'No email';
  
  elements.contactResults.classList.add('hidden');
  elements.selectedContact.classList.remove('hidden');
  elements.contactSearch.value = '';
  
  updateSendButton();
}

function clearSelectedContact() {
  selectedContact = null;
  elements.selectedContact.classList.add('hidden');
  updateSendButton();
}

// Send to Webhook
async function sendToWebhook() {
  const settings = await chrome.storage.sync.get(['webhookUrl']);
  
  if (!settings.webhookUrl) {
    showStatus('Configure webhook URL first', 'error');
    toggleSettings();
    return;
  }
  
  if (!currentScreenshot || !selectedContact) {
    showStatus('Capture screenshot and select contact first', 'error');
    return;
  }
  
  try {
    showStatus('Sending to n8n...', 'loading');
    elements.sendBtn.disabled = true;
    
    // Prepare payload
    const payload = {
      screenshot: currentScreenshot, // base64 data URL
      source: {
        url: currentTabInfo.url,
        domain: currentTabInfo.domain,
        title: currentTabInfo.title
      },
      contact: {
        id: selectedContact.id,
        name: selectedContact.name,
        email: selectedContact.email
      },
      note: elements.taskNote.value.trim(),
      capturedAt: new Date().toISOString()
    };
    
    // Send to n8n webhook
    const response = await fetch(settings.webhookUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload)
    });
    
    if (!response.ok) {
      throw new Error(`Webhook error: ${response.status}`);
    }
    
    showStatus('Task created successfully!', 'success');
    
    // Reset form
    setTimeout(() => {
      resetForm();
    }, 1500);
    
  } catch (error) {
    console.error('Webhook error:', error);
    showStatus(`Failed: ${error.message}`, 'error');
    elements.sendBtn.disabled = false;
  }
}

// UI Helpers
function updateSendButton() {
  elements.sendBtn.disabled = !(currentScreenshot && selectedContact);
}

function showStatus(message, type) {
  elements.statusMessage.textContent = message;
  elements.statusMessage.className = `status-message ${type}`;
}

function hideStatus() {
  elements.statusMessage.classList.add('hidden');
}

function resetForm() {
  currentScreenshot = null;
  currentTabInfo = null;
  selectedContact = null;
  
  elements.screenshotPlaceholder.style.display = 'flex';
  elements.screenshotPreview.classList.add('hidden');
  elements.screenshotPreview.src = '';
  elements.pageInfo.classList.add('hidden');
  elements.selectedContact.classList.add('hidden');
  elements.taskNote.value = '';
  elements.sendBtn.disabled = true;
  
  hideStatus();
}

function getInitials(name) {
  return name
    .split(' ')
    .map(n => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

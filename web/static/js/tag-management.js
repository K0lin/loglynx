// IP Tag Management JavaScript
let currentIPEditing = null;
let ipTagsCache = new Map();
let lastBulkFetch = 0;
const CACHE_DURATION = 30000; // 30 seconds cache for bulk fetch

// Load tagging state from localStorage
function loadTaggingState() {
  const enabled = localStorage.getItem('loglynx_ip_tagging_enabled') === 'true';
  const checkbox = document.getElementById('enable-tagging');
  if (checkbox) {
    checkbox.checked = enabled;
  }
  
  toggleTagging(enabled);
}

// Save tagging state to localStorage
function saveTaggingState(enabled) {
  localStorage.setItem('loglynx_ip_tagging_enabled', enabled);
}

function openTagModal(ip) {
  currentIPEditing = ip;

  // Load existing tag data from cache if available, otherwise fetch
  if (ipTagsCache.has(ip)) {
      populateAndShowModal(ip, ipTagsCache.get(ip));
  } else {
      fetch(`/api/v1/ip/tags/${ip}`)
        .then(response => response.json())
        .then(data => {
          // Normalize data from API (handle both PascalCase and snake_case)
          const normalizedData = {
              ip_address: data.IPAddress || data.ip_address || ip,
              friendly_name: data.FriendlyName || data.friendly_name || '',
              tags: data.Tags || data.tags || ''
          };
          ipTagsCache.set(ip, normalizedData);
          populateAndShowModal(ip, normalizedData);
        })
        .catch(err => {
          console.error('Failed to load tag data:', err);
          populateAndShowModal(ip, { friendly_name: '', tags: '' });
        });
  }
}

function populateAndShowModal(ip, data) {
    const nameInput = document.getElementById('friendly-name');
    const tagsInput = document.getElementById('tags-input');
    
    // Support both PascalCase and snake_case
    if (nameInput) nameInput.value = data.FriendlyName || data.friendly_name || '';
    if (tagsInput) tagsInput.value = data.Tags || data.tags || '';
    
    const modalElement = document.getElementById('tag-modal');
    if (modalElement) {
        const modal = bootstrap.Modal.getOrCreateInstance(modalElement);
        modal.show();
    }
}

function closeTagModal() {
  const modalElement = document.getElementById('tag-modal');
  if (modalElement) {
      const modal = bootstrap.Modal.getInstance(modalElement);
      if (modal) modal.hide();
  }
  currentIPEditing = null;
}

function saveTag() {
  const friendlyName = document.getElementById('friendly-name').value;
  const tags = document.getElementById('tags-input').value;
  const ip = currentIPEditing;

  fetch('/api/v1/ip/tags', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      ip_address: ip,
      friendly_name: friendlyName,
      tags: tags
    })
  })
  .then(response => {
    if (!response.ok) {
      throw new Error('Failed to save tag');
    }
    return response.json();
  })
  .then(data => {
    ipTagsCache.set(ip, data);
    closeTagModal();
    // Update all UI elements for this IP
    updateUIForIP(ip, data);
  })
  .catch(err => {
    console.error('Failed to save tag:', err);
    alert('Failed to save tag. Please try again.');
  });
}

// Update all UI elements for a specific IP
function updateUIForIP(ip, data) {
  const enabled = localStorage.getItem('loglynx_ip_tagging_enabled') === 'true';
  
  // Normalize fields from input data
  const friendlyName = data.FriendlyName || data.friendly_name || '';
  const tagsStr = data.Tags || data.tags || '';

  // 1. Update IP display spans
  const ipDisplays = document.querySelectorAll(`.ip-display[data-ip="${ip}"]`);
  ipDisplays.forEach(display => {
      // Store original HTML if not already stored
      if (!display.dataset.originalHtml) {
          display.dataset.originalHtml = display.innerHTML;
      }

      if (enabled && friendlyName) {
          display.innerHTML = `<strong>${friendlyName}</strong> <small class="text-muted">(${ip})</small>`;
      } else {
          // Restore original HTML (link, code, etc.)
          display.innerHTML = display.dataset.originalHtml;
      }
  });

  // 2. Update tag chips containers
  /*const containers = document.querySelectorAll(`.tag-chips[data-ip="${ip}"]`);
  
  containers.forEach(container => {
    container.innerHTML = '';
    
    if (!enabled) return;

    if (tagsStr) {
      const tags = tagsStr.split(',').map(tag => tag.trim()).filter(tag => tag);
      tags.forEach(tag => {
        const chip = document.createElement('span');
        chip.textContent = tag;
        chip.className = 'badge rounded-pill bg-info text-dark me-1';
        chip.style.fontSize = '10px';
        chip.style.cursor = 'pointer';
        chip.onclick = (e) => {
          e.stopPropagation();
          openTagModal(ip);
        };
        container.appendChild(chip);
      });
    }
  });*/

  // 3. Update edit buttons visibility
  const editBtns = document.querySelectorAll(`.edit-tag-btn[data-ip="${ip}"]`);
  editBtns.forEach(btn => {
    btn.style.display = enabled ? 'inline-block' : 'none';
  });
}

// Load and display tags for an IP
function loadTagsForIP(ip) {
  if (ipTagsCache.has(ip)) {
    updateUIForIP(ip, ipTagsCache.get(ip));
    return;
  }

  fetch(`/api/v1/ip/tags/${ip}`)
    .then(response => response.json())
    .then(data => {
      // Standardize data from API
      const normalizedData = {
          ip_address: data.IPAddress || data.ip_address || ip,
          friendly_name: data.FriendlyName || data.friendly_name || '',
          tags: data.Tags || data.tags || ''
      };
      ipTagsCache.set(ip, normalizedData);
      updateUIForIP(ip, normalizedData);
    })
    .catch(err => console.error('Failed to load tags for ' + ip + ':', err));
}

// Toggle tagging display
function toggleTagging(enabled) {
  saveTaggingState(enabled);
  
  if (enabled) {
    const now = Date.now();
    const shouldFetchBulk = (now - lastBulkFetch) > CACHE_DURATION;

    if (shouldFetchBulk) {
        lastBulkFetch = now;
        fetch('/api/v1/ip/tags')
          .then(response => response.json())
          .then(tags => {
            ipTagsCache.clear();
            tags.forEach(tag => {
                // Normalize each tag from bulk API
                const ip = tag.IPAddress || tag.ip_address;
                if (ip) {
                    ipTagsCache.set(ip, {
                        ip_address: ip,
                        friendly_name: tag.FriendlyName || tag.friendly_name || '',
                        tags: tag.Tags || tag.tags || ''
                    });
                }
            });
            applyTagsFromCache();
          })
          .catch(err => {
            console.error('Failed to load all tags:', err);
            applyTagsFromCache();
          });
    } else {
        applyTagsFromCache();
    }
  } else {
    // Restore original IP displays and hide everything else
    const ipDisplays = document.querySelectorAll('.ip-display');
    ipDisplays.forEach(display => {
        if (display.dataset.originalHtml) {
            display.innerHTML = display.dataset.originalHtml;
        }
    });

    const containers = document.querySelectorAll('.tag-chips');
    containers.forEach(c => c.innerHTML = '');
    
    const editBtns = document.querySelectorAll('.edit-tag-btn');
    editBtns.forEach(b => b.style.display = 'none');
  }
}

function applyTagsFromCache() {
    const ipDisplays = document.querySelectorAll('.ip-display');
    const uniqueIPs = new Set();
    ipDisplays.forEach(display => {
      const ip = display.dataset.ip;
      if (ip) uniqueIPs.add(ip);
    });

    uniqueIPs.forEach(ip => {
      const data = ipTagsCache.get(ip) || { friendly_name: '', tags: '' };
      updateUIForIP(ip, data);
    });
}

// Initialize tagging state on page load
document.addEventListener('DOMContentLoaded', () => {
  loadTaggingState();
});

// Handle Bootstrap modal events to cleanup
document.addEventListener('hidden.bs.modal', function (event) {
  if (event && event.target && event.target.id === 'tag-modal') {
    currentIPEditing = null;
  }
});

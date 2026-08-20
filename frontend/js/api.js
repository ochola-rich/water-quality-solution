const BASE_URL = window.location.origin || 'http://localhost:3000';

async function apiFetch(endpoint, options = {}) {
  const url = endpoint.startsWith('http') ? endpoint : `${BASE_URL}${endpoint}`;
  
  const defaultHeaders = {
    'Accept': 'application/json',
  };
  
  if (!(options.body instanceof FormData)) {
    defaultHeaders['Content-Type'] = 'application/json';
  }
  
  const config = {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
  };
  
  if (options.body && !(options.body instanceof FormData) && typeof options.body !== 'string') {
    config.body = JSON.stringify(options.body);
  }
  
  try {
    const response = await fetch(url, config);
    
    if (!response.ok) {
      let errorMessage = `Request failed with status ${response.status}`;
      try {
        const errorData = await response.json();
        if (errorData.message) errorMessage = errorData.message;
        if (errorData.error) errorMessage = errorData.error;
      } catch (e) {
        errorMessage = response.statusText || errorMessage;
      }
      throw new Error(errorMessage);
    }
    
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      return await response.json();
    }
    
    return await response.text();
  } catch (error) {
    if (error.message === 'Failed to fetch') {
      throw new Error('Network error — are you connected to the backend?');
    }
    throw error;
  }
}

// --- Public API methods ---

/**
 * GET request
 * @param {string} endpoint - API path (e.g., '/api/reports')
 * @returns {Promise<any>} Parsed JSON response
 */
function get(endpoint) {
  return apiFetch(endpoint, { method: 'GET' });
}

/**
 * POST request with JSON body
 * @param {string} endpoint - API path
 * @param {object} body - JSON-serializable object
 * @returns {Promise<any>} Parsed JSON response
 */
function post(endpoint, body) {
  return apiFetch(endpoint, {
    method: 'POST',
    body,
  });
}

/**
 * POST request with FormData (for file uploads)
 * @param {string} endpoint - API path
 * @param {FormData} formData - Multipart form data
 * @returns {Promise<any>} Parsed JSON response
 */
function postMultipart(endpoint, formData) {
  return apiFetch(endpoint, {
    method: 'POST',
    body: formData,
  });
}

/**
 * PUT request with JSON body
 * @param {string} endpoint - API path
 * @param {object} body - JSON-serializable object
 * @returns {Promise<any>} Parsed JSON response
 */
function put(endpoint, body) {
  return apiFetch(endpoint, {
    method: 'PUT',
    body,
  });
}

/**
 * DELETE request
 * @param {string} endpoint - API path
 * @returns {Promise<any>} Parsed JSON response
 */
function del(endpoint) {
  return apiFetch(endpoint, { method: 'DELETE' });
}

// --- Export ---
export const api = {
  get,
  post,
  postMultipart,
  put,
  delete: del,
  submitReport: (formData) => postMultipart('/api/reports', formData),
};

if (typeof window !== 'undefined') {
  window.api = api;
}
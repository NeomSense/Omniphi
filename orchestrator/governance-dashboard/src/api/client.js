/**
 * HTTP client for Cosmos REST (LCD) and Guard module endpoints.
 * All endpoint paths are centralized here for easy adaptation.
 */

const LCD_URL = (import.meta.env.VITE_LCD_URL || 'http://localhost:1317').replace(/\/+$/, '');
const GUARD_URL = (import.meta.env.VITE_GUARD_REST_URL || LCD_URL).replace(/\/+$/, '');
const TIMEOUT_MS = 10_000;

export function lcdUrl(path) {
  return `${LCD_URL}${path}`;
}

export function guardUrl(path) {
  return `${GUARD_URL}${path}`;
}

/**
 * GET JSON with timeout via AbortController.
 * Returns parsed JSON or throws with a readable message.
 */
export async function getJSON(url) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch(url, { signal: controller.signal });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      const msg = text ? `${res.status} – ${text.slice(0, 200)}` : `${res.status} ${res.statusText}`;
      throw new Error(msg);
    }
    return await res.json();
  } catch (err) {
    if (err.name === 'AbortError') {
      throw new Error(`Request timed out after ${TIMEOUT_MS}ms: ${url}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * POST JSON to the backend actions proxy.
 * Includes X-API-Key header if VITE_BACKEND_API_KEY is set.
 */
export async function postJSON(url, body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  const headers = { 'Content-Type': 'application/json' };
  const apiKey = import.meta.env.VITE_BACKEND_API_KEY;
  if (apiKey) {
    headers['X-API-Key'] = apiKey;
  }

  try {
    const res = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: controller.signal,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(text || `${res.status} ${res.statusText}`);
    }
    return await res.json();
  } catch (err) {
    if (err.name === 'AbortError') {
      throw new Error(`Request timed out after ${TIMEOUT_MS}ms: ${url}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

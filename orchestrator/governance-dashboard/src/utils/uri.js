/**
 * URI normalization for advisory report links.
 *
 * Reports can be stored via HTTP, IPFS, or local file paths.
 * This module resolves each scheme to a fetchable URL or marks
 * it as unsupported with a user-facing explanation.
 */

const IPFS_GATEWAY = (import.meta.env.VITE_IPFS_GATEWAY || 'https://ipfs.io/ipfs/').replace(/\/+$/, '/');

/**
 * Analyze a report URI and return fetch instructions.
 *
 * @param {string} uri  Raw URI from on-chain AdvisoryLink
 * @returns {{ kind: string, fetchUrl?: string, note?: string }}
 */
export function normalizeReportUri(uri) {
  if (!uri) {
    return { kind: 'unknown', note: 'No URI provided.' };
  }

  const trimmed = uri.trim();

  // HTTP / HTTPS — fetch directly
  if (/^https?:\/\//i.test(trimmed)) {
    return { kind: trimmed.toLowerCase().startsWith('https') ? 'https' : 'http', fetchUrl: trimmed };
  }

  // IPFS — resolve through gateway
  if (/^ipfs:\/\//i.test(trimmed)) {
    const cid = trimmed.replace(/^ipfs:\/\//i, '');
    return { kind: 'ipfs', fetchUrl: `${IPFS_GATEWAY}${cid}` };
  }

  // file:// — browsers cannot fetch local files
  if (/^file:\/\//i.test(trimmed)) {
    return {
      kind: 'file',
      note: 'Browsers cannot fetch file:// URIs. To verify this report, download it manually and run: sha256sum <file>. Consider re-publishing via HTTP or IPFS.',
    };
  }

  // Bare CID (starts with Qm or bafy) — treat as IPFS
  if (/^(Qm[1-9A-HJ-NP-Za-km-z]{44,}|bafy[a-z2-7]{50,})/.test(trimmed)) {
    return { kind: 'ipfs', fetchUrl: `${IPFS_GATEWAY}${trimmed}` };
  }

  return {
    kind: 'unknown',
    note: `Unrecognized URI scheme. Cannot auto-verify. URI: ${trimmed}`,
  };
}

/**
 * Fetch raw bytes from a report URL with timeout.
 * Returns the ArrayBuffer on success.
 *
 * @param {string} url
 * @param {number} timeoutMs
 * @returns {Promise<ArrayBuffer>}
 */
export async function fetchReportBytes(url, timeoutMs = 15_000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const res = await fetch(url, { signal: controller.signal });
    if (!res.ok) {
      throw new Error(`${res.status} ${res.statusText}`);
    }
    return await res.arrayBuffer();
  } catch (err) {
    if (err.name === 'AbortError') {
      throw new Error(`Fetch timed out after ${timeoutMs / 1000}s`);
    }
    // Provide actionable CORS hint
    if (err instanceof TypeError && err.message.includes('fetch')) {
      throw new Error('Network error — likely a CORS restriction or the URI is unreachable.');
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

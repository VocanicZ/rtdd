// Fetching a release: resolving the tag, downloading the asset, and proving it is the one
// GoReleaser published.
import crypto from 'node:crypto';

export const REPO = 'VocanicZ/rtdd';

export function baseUrl(env = process.env) {
  return env.RTDD_BASE_URL || `https://github.com/${REPO}/releases/download`;
}

export function apiUrl(env = process.env) {
  return env.RTDD_API_URL || `https://api.github.com/repos/${REPO}/releases/latest`;
}

/**
 * fetchBuffer downloads a URL and returns its bytes, failing loudly on any non-2xx.
 *
 * A redirect is followed by fetch itself. A 404 here is the ordinary "that release or that
 * asset does not exist" case, so the message names the URL rather than the status alone.
 */
export async function fetchBuffer(url, fetchImpl = fetch) {
  const res = await fetchImpl(url, { redirect: 'follow' });
  if (!res.ok) {
    throw new Error(`GET ${url} failed: ${res.status} ${res.statusText}`);
  }
  return Buffer.from(await res.arrayBuffer());
}

/**
 * resolveVersion returns the tag to install.
 *
 * An explicit tag wins and is never compared against anything: pinning is an override, the
 * same way `rtdd update --version` is, and that is how a bad release gets rolled back.
 */
export async function resolveVersion({ version, env = process.env, fetchImpl = fetch } = {}) {
  if (version) return version;
  const url = apiUrl(env);
  const body = await fetchBuffer(url, fetchImpl);
  let payload;
  try {
    payload = JSON.parse(body.toString('utf8'));
  } catch {
    throw new Error(`could not resolve the latest release version from ${url}: the response was not valid JSON`);
  }
  const tag = payload && payload.tag_name;
  if (!tag) throw new Error(`could not resolve the latest release version from ${url}`);
  return tag;
}

export function sha256(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex');
}

/**
 * expectedChecksum pulls one archive's digest out of a checksums.txt body.
 *
 * The name is matched on a whole trailing field rather than with `includes`, so
 * `rtdd_1.2.3_linux_amd64.tar.gz` cannot be satisfied by a line for some other asset that
 * merely contains that string.
 */
export function expectedChecksum(checksums, archive) {
  for (const line of String(checksums).split('\n')) {
    const trimmed = line.trim();
    if (trimmed === '') continue;
    const parts = trimmed.split(/\s+/);
    if (parts.length >= 2 && parts[parts.length - 1] === archive) return parts[0];
  }
  return null;
}

/**
 * verify throws unless the bytes hash to what checksums.txt claims.
 *
 * This is the only thing standing between a piped installer and running whatever a
 * compromised mirror served, so a missing entry is a failure rather than a skip.
 */
export function verify(bytes, checksums, archive) {
  const want = expectedChecksum(checksums, archive);
  if (!want) throw new Error(`no checksum entry for ${archive} in checksums.txt`);
  const got = sha256(bytes);
  if (want.toLowerCase() !== got.toLowerCase()) {
    throw new Error(`checksum mismatch for ${archive}: expected ${want}, got ${got}`);
  }
}

// Adding the install directory to the Windows user PATH.
//
// Unix installers put the binary somewhere already on PATH. Windows has no such place that
// does not need admin rights, so the per-user PATH in the registry is edited instead.
//
// `reg` is used rather than `setx`, which silently truncates any value past 1024 characters
// — a real way to destroy someone's PATH while reporting success.
import { execFileSync } from 'node:child_process';
import path from 'node:path';

const KEY = 'HKCU\\Environment';

// The registry tolerates far more, but a user PATH beyond this is already pathological and
// rewriting it wholesale is the kind of edit that is unrecoverable if it goes wrong.
const MAX_PATH_LENGTH = 8000;

/**
 * readUserPath returns the current per-user PATH, or '' when the value does not exist.
 *
 * A fresh Windows account genuinely has no HKCU\Environment\Path, and `reg query` exits
 * non-zero for it. That is an empty PATH, not an error.
 */
export function readUserPath(run = execFileSync) {
  let out;
  try {
    out = run('reg', ['query', KEY, '/v', 'Path'], { encoding: 'utf8' });
  } catch {
    return '';
  }
  const match = String(out).match(/^\s*Path\s+REG_(?:EXPAND_)?SZ\s+(.*)$/im);
  return match ? match[1].trim() : '';
}

/** samePath compares two directories the way Windows does: case-insensitively. */
function samePath(a, b) {
  return path.normalize(a).replace(/\\+$/, '').toLowerCase() === path.normalize(b).replace(/\\+$/, '').toLowerCase();
}

/**
 * addToUserPath appends dir to the per-user PATH unless it is already there.
 *
 * The already-there check is what makes re-running the installer safe: without it, every
 * run would grow PATH by one more copy of the same directory.
 *
 * @returns {boolean} true when it changed the PATH, false when nothing was needed
 */
export function addToUserPath(dir, run = execFileSync) {
  const current = readUserPath(run);
  const parts = current.split(';').filter((p) => p !== '');
  if (parts.some((p) => samePath(p, dir))) return false;

  const next = [...parts, dir].join(';');
  if (next.length > MAX_PATH_LENGTH) {
    throw new Error(
      `your user PATH is ${current.length} characters; adding ${dir} would exceed ${MAX_PATH_LENGTH} and risk truncating it`
    );
  }
  // REG_EXPAND_SZ, not REG_SZ: an existing PATH usually contains %USERPROFILE% and friends,
  // and rewriting it as a plain string would stop those expanding.
  run('reg', ['add', KEY, '/v', 'Path', '/t', 'REG_EXPAND_SZ', '/d', next, '/f'], { stdio: 'ignore' });
  return true;
}

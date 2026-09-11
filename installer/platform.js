// Platform facts the installer needs: which release asset to fetch, what the binary is
// called, and where it goes. Every OS difference in the installer is in this file.
import os from 'node:os';
import path from 'node:path';
import fs from 'node:fs';

/**
 * goos maps Node's process.platform onto the GOOS in a release asset's name.
 * @param {string} platform
 */
export function goosFor(platform) {
  switch (platform) {
    case 'linux':
      return 'linux';
    case 'darwin':
      return 'darwin';
    case 'win32':
      return 'windows';
    default:
      throw new Error(`unsupported OS: ${platform}; rtdd ships linux, darwin and windows binaries only`);
  }
}

/**
 * goarch maps Node's process.arch onto the GOARCH in a release asset's name.
 * @param {string} arch
 */
export function goarchFor(arch) {
  switch (arch) {
    case 'x64':
      return 'amd64';
    case 'arm64':
      return 'arm64';
    default:
      throw new Error(`unsupported architecture: ${arch}; rtdd ships amd64 and arm64 binaries only`);
  }
}

/**
 * target resolves the os/arch pair, rejecting the one combination the release matrix skips.
 * @param {string} platform
 * @param {string} arch
 */
export function target(platform = process.platform, arch = process.arch) {
  const goos = goosFor(platform);
  const goarch = goarchFor(arch);
  // .goreleaser.yaml ignores windows/arm64, so there is no asset to fetch. Saying so here
  // beats a 404 halfway through the download.
  if (goos === 'windows' && goarch === 'arm64') {
    throw new Error('rtdd does not ship a windows/arm64 binary; build from source with: go build ./cmd/rtdd');
  }
  return { goos, goarch };
}

/** binaryName is what the binary is called, inside the archive and on disk. */
export function binaryName(goos) {
  return goos === 'windows' ? 'rtdd.exe' : 'rtdd';
}

/**
 * archiveName mirrors .goreleaser.yaml's name_template.
 *
 * Always .tar.gz, including on Windows, where a .zip is also published for people
 * downloading by hand: this installer extracts in pure Node, and tar is the format it can
 * read without depending on anything the OS may or may not ship.
 */
export function archiveName(version, goos, goarch) {
  const num = String(version).replace(/^v/, '');
  return `rtdd_${num}_${goos}_${goarch}.tar.gz`;
}

/** writable reports whether a directory exists and can be written to. */
function writable(dir) {
  try {
    fs.accessSync(dir, fs.constants.W_OK);
    return fs.statSync(dir).isDirectory();
  } catch {
    return false;
  }
}

/**
 * defaultInstallDir picks where the binary goes when RTDD_INSTALL_DIR says nothing.
 *
 * Windows has no /usr/local/bin, so it gets the per-user location that needs no admin
 * rights. Unix prefers /usr/local/bin when it is already writable and falls back to
 * ~/.local/bin, which is the rule the old shell installer used.
 *
 * @param {string} goos
 * @param {NodeJS.ProcessEnv} env
 */
export function defaultInstallDir(goos, env = process.env, home = os.homedir()) {
  if (goos === 'windows') {
    const base = env.LOCALAPPDATA || path.join(home, 'AppData', 'Local');
    return path.join(base, 'Programs', 'rtdd');
  }
  if (writable('/usr/local/bin')) return '/usr/local/bin';
  return path.join(home, '.local', 'bin');
}

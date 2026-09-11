#!/usr/bin/env node
// The rtdd installer.
//
//   npx github:VocanicZ/rtdd
//
// One command for Linux, macOS and Windows. It is written in Node rather than as a shell
// script because no shell script can be that one command: `curl … | sh` needs a POSIX shell
// stock Windows does not have, and `irm … | iex` needs a PowerShell that Linux and macOS do
// not have. `npx` is a program rather than shell syntax, so the same string runs in bash,
// PowerShell and cmd alike.
//
// It downloads the release binary for this platform, checks it against the published
// checksums, puts it on PATH, and installs the machine-wide agent skill.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

import { target, binaryName, archiveName, defaultInstallDir } from './platform.js';
import { baseUrl, resolveVersion, fetchBuffer, verify } from './release.js';
import { extractFile } from './targz.js';
import { addToUserPath } from './winpath.js';

const USAGE = `rtdd installer

usage:
  npx github:VocanicZ/rtdd [options]

options:
  --version=<tag>       install this release tag instead of the latest
  --install-dir=<path>  where to put the binary
  --no-skill            install the binary only, without the machine-wide agent skill
  -h, --help            show this

environment:
  RTDD_VERSION, RTDD_INSTALL_DIR, RTDD_NO_SKILL   the same three, as variables
`;

/**
 * parseArgs turns flags and environment into one options object.
 *
 * Flags win over environment variables, because a flag is typed at the moment of use and a
 * variable may be left over from something else in the same shell.
 */
export function parseArgs(argv = [], env = process.env) {
  const opts = {
    help: false,
    version: env.RTDD_VERSION || '',
    installDir: env.RTDD_INSTALL_DIR || '',
    skill: !env.RTDD_NO_SKILL,
  };
  for (const arg of argv) {
    if (arg === '-h' || arg === '--help') opts.help = true;
    else if (arg === '--no-skill') opts.skill = false;
    else if (arg.startsWith('--version=')) opts.version = arg.slice('--version='.length);
    else if (arg.startsWith('--install-dir=')) opts.installDir = arg.slice('--install-dir='.length);
    else throw new Error(`unknown option ${arg}\n\n${USAGE}`);
  }
  return opts;
}

function log(message) {
  process.stderr.write(`${message}\n`);
}

/**
 * install performs the whole install and returns where the binary landed.
 *
 * Exported so the tests can drive it against a local fixture server without going through
 * process exit codes.
 */
export async function install({ argv = [], env = process.env, fetchImpl = fetch } = {}) {
  const opts = parseArgs(argv, env);
  if (opts.help) {
    process.stdout.write(USAGE);
    return null;
  }

  const { goos, goarch } = target();
  const bin = binaryName(goos);

  let version = opts.version;
  if (!version) {
    log('resolving the latest rtdd release...');
    version = await resolveVersion({ env, fetchImpl });
  }

  const archive = archiveName(version, goos, goarch);
  const base = baseUrl(env);

  log(`downloading ${archive} (${version})...`);
  const [archiveBytes, checksums] = await Promise.all([
    fetchBuffer(`${base}/${version}/${archive}`, fetchImpl),
    fetchBuffer(`${base}/${version}/checksums.txt`, fetchImpl),
  ]);

  log('verifying checksum...');
  verify(archiveBytes, checksums.toString('utf8'), archive);

  log('extracting...');
  const binary = extractFile(archiveBytes, bin);

  const installDir = opts.installDir || defaultInstallDir(goos, env);
  fs.mkdirSync(installDir, { recursive: true });
  const dest = path.join(installDir, bin);
  // Written to a neighbouring temp name and renamed, so a half-written binary never sits at
  // the path the user is about to run. rename within one directory is atomic.
  const tmp = `${dest}.tmp-${process.pid}`;
  fs.writeFileSync(tmp, binary, { mode: 0o755 });
  fs.renameSync(tmp, dest);
  log(`rtdd ${version} installed to ${dest}`);

  if (goos === 'windows') {
    // Non-fatal: the binary is installed and runnable by its full path either way, and a
    // PATH edit that fails is not a reason to report the install as failed.
    try {
      if (addToUserPath(installDir)) {
        log(`added ${installDir} to your user PATH — open a new terminal for it to take effect`);
      }
    } catch (err) {
      log(`note: could not add ${installDir} to your PATH: ${err.message}`);
      log(`      add it by hand, or run rtdd as ${dest}`);
    }
  }

  if (opts.skill) {
    installSkill(dest);
  }

  return dest;
}

/**
 * installSkill runs the freshly installed binary's own `skill install`.
 *
 * The binary does this rather than the installer writing the files, because the agent
 * front-ends are generated from protocol/PROTOCOL.md and embedded in the binary; a copy
 * living here would be a second source of truth that silently goes stale.
 *
 * Non-fatal, like the PATH edit: the binary install is what the user asked for and it has
 * already succeeded.
 */
function installSkill(binaryPath, run = execFileSync) {
  try {
    run(binaryPath, ['skill', 'install'], { stdio: 'inherit' });
  } catch (err) {
    log('note: could not install the agent skill; rtdd itself is installed and usable.');
    log("      run 'rtdd skill install' to retry, or 'rtdd skill prompt' to install it by hand.");
  }
}

/**
 * isMain reports whether this file is the entrypoint rather than an import.
 *
 * Both sides are resolved through realpath because npm installs a bin as a SYMLINK in
 * node_modules/.bin — which is exactly how `npx github:VocanicZ/rtdd` runs this. Comparing
 * process.argv[1] to import.meta.url directly is false through that symlink, and the
 * installer then exits silently having done nothing, on the only path a user actually takes.
 *
 * fileURLToPath rather than stripping a `file://` prefix by hand: on Windows the URL is
 * file:///C:/... and hand-slicing leaves a leading slash that resolves to nothing.
 */
function isMain() {
  if (!process.argv[1]) return false;
  try {
    return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url));
  } catch {
    return false;
  }
}

if (isMain()) {
  install({ argv: process.argv.slice(2) }).catch((err) => {
    process.stderr.write(`rtdd installer: ${err.message}\n`);
    process.exit(1);
  });
}

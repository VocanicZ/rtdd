import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';

import { goosFor, goarchFor, target, binaryName, archiveName, defaultInstallDir } from './platform.js';
import { expectedChecksum, verify, sha256, resolveVersion } from './release.js';
import { extractFile, listEntries } from './targz.js';
import { addToUserPath, readUserPath } from './winpath.js';
import { parseArgs, install } from './index.js';

// ---------------------------------------------------------------- test helpers

// tarball builds a single-member .tar.gz by hand, so the extractor is tested against bytes
// this repository produced rather than against whatever `tar` happens to be installed.
function tarball(name, contents) {
  const data = Buffer.from(contents);
  const header = Buffer.alloc(512);
  header.write(name, 0, 100, 'utf8');
  header.write('000755 \0', 100, 8, 'utf8'); // mode
  header.write('000000 \0', 108, 8, 'utf8'); // uid
  header.write('000000 \0', 116, 8, 'utf8'); // gid
  header.write(data.length.toString(8).padStart(11, '0') + '\0', 124, 12, 'utf8');
  header.write('00000000000\0', 136, 12, 'utf8'); // mtime
  header.write('        ', 148, 8, 'utf8'); // checksum field, spaces while summing
  header.write('0', 156, 1, 'utf8'); // typeflag: regular file
  header.write('ustar\0' + '00', 257, 8, 'utf8');

  let sum = 0;
  for (const byte of header) sum += byte;
  header.write(sum.toString(8).padStart(6, '0') + '\0 ', 148, 8, 'utf8');

  const padded = Buffer.alloc(Math.ceil(data.length / 512) * 512);
  data.copy(padded);
  // Two zero blocks end the archive.
  return zlib.gzipSync(Buffer.concat([header, padded, Buffer.alloc(1024)]));
}

// serveRelease stands in for the GitHub release endpoints, on localhost.
async function serveRelease({ tag, archive, bytes, checksums, apiBody }) {
  const server = http.createServer((req, res) => {
    const routes = {
      [`/${tag}/${archive}`]: bytes,
      [`/${tag}/checksums.txt`]: Buffer.from(checksums),
      '/releases/latest': apiBody ? Buffer.from(apiBody) : null,
    };
    const body = routes[req.url];
    if (!body) {
      res.writeHead(404);
      res.end('not found');
      return;
    }
    res.writeHead(200);
    res.end(body);
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  return { url: `http://127.0.0.1:${port}`, close: () => new Promise((r) => server.close(r)) };
}

function tempDir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

// ---------------------------------------------------------------- platform

test('platform maps Node names onto release asset names', () => {
  assert.equal(goosFor('linux'), 'linux');
  assert.equal(goosFor('darwin'), 'darwin');
  assert.equal(goosFor('win32'), 'windows');
  assert.equal(goarchFor('x64'), 'amd64');
  assert.equal(goarchFor('arm64'), 'arm64');
});

test('platform rejects an OS or architecture with no published binary', () => {
  assert.throws(() => goosFor('aix'), /unsupported OS/);
  assert.throws(() => goarchFor('ia32'), /unsupported architecture/);
});

// .goreleaser.yaml's ignore list excludes windows/arm64, so there is nothing to download.
// Failing here beats a 404 partway through an install.
test('platform rejects windows/arm64, which the release matrix does not build', () => {
  assert.throws(() => target('win32', 'arm64'), /does not ship a windows\/arm64 binary/);
});

test('archiveName mirrors the goreleaser name_template and drops the leading v', () => {
  assert.equal(archiveName('v1.2.3', 'linux', 'amd64'), 'rtdd_1.2.3_linux_amd64.tar.gz');
  assert.equal(archiveName('1.2.3', 'windows', 'amd64'), 'rtdd_1.2.3_windows_amd64.tar.gz');
});

test('binaryName is rtdd.exe on Windows only', () => {
  assert.equal(binaryName('windows'), 'rtdd.exe');
  assert.equal(binaryName('linux'), 'rtdd');
});

test('the Windows default install directory needs no admin rights', () => {
  const dir = defaultInstallDir('windows', { LOCALAPPDATA: 'C:\\Users\\me\\AppData\\Local' });
  assert.equal(dir, path.join('C:\\Users\\me\\AppData\\Local', 'Programs', 'rtdd'));
});

// ---------------------------------------------------------------- targz

test('extractFile pulls a named member out of a tar.gz', () => {
  const gz = tarball('rtdd', 'binary contents here');
  assert.equal(extractFile(gz, 'rtdd').toString(), 'binary contents here');
});

test('extractFile reads a member whose size is not a multiple of the block size', () => {
  // 513 bytes spans two blocks and leaves 511 bytes of padding: reading the padding back as
  // content is the classic tar bug, and it would corrupt every binary this installs.
  const body = 'x'.repeat(513);
  const gz = tarball('rtdd', body);
  const got = extractFile(gz, 'rtdd').toString();
  assert.equal(got.length, 513);
  assert.equal(got, body);
});

test('extractFile names what the archive actually held when the member is missing', () => {
  const gz = tarball('rtdd', 'x');
  assert.throws(() => extractFile(gz, 'rtdd.exe'), /does not contain rtdd\.exe.*rtdd/s);
});

test('listEntries stops at the end-of-archive marker rather than reading padding', () => {
  assert.equal(listEntries(zlib.gunzipSync(tarball('rtdd', 'x'))).length, 1);
});

// ---------------------------------------------------------------- release

test('expectedChecksum matches the whole filename field, not a substring', () => {
  const sums = [
    'aaaa  rtdd_1.0.0_linux_arm64.tar.gz',
    'bbbb  rtdd_1.0.0_linux_amd64.tar.gz',
    'cccc  other_rtdd_1.0.0_linux_amd64.tar.gz.sig',
  ].join('\n');
  assert.equal(expectedChecksum(sums, 'rtdd_1.0.0_linux_amd64.tar.gz'), 'bbbb');
});

test('expectedChecksum reports nothing rather than guessing when the asset is absent', () => {
  assert.equal(expectedChecksum('aaaa  something-else.tar.gz', 'rtdd_1.0.0_linux_amd64.tar.gz'), null);
});

// The checksum is all that stands between a piped installer and running whatever a
// compromised mirror served, so a missing entry has to fail rather than skip the check.
test('verify refuses an archive with no checksum entry', () => {
  assert.throws(() => verify(Buffer.from('x'), 'aaaa  other.tar.gz', 'rtdd.tar.gz'), /no checksum entry/);
});

test('verify refuses an archive whose bytes do not match the published digest', () => {
  const bytes = Buffer.from('tampered');
  const sums = `${sha256(Buffer.from('original'))}  rtdd.tar.gz`;
  assert.throws(() => verify(bytes, sums, 'rtdd.tar.gz'), /checksum mismatch/);
});

test('verify accepts the bytes the checksum was computed over', () => {
  const bytes = Buffer.from('original');
  verify(bytes, `${sha256(bytes)}  rtdd.tar.gz`, 'rtdd.tar.gz');
});

test('resolveVersion prefers an explicit tag over asking the API at all', async () => {
  const version = await resolveVersion({
    version: 'v9.9.9',
    fetchImpl: () => {
      throw new Error('the API must not be consulted when a tag is pinned');
    },
  });
  assert.equal(version, 'v9.9.9');
});

// ---------------------------------------------------------------- winpath

test('addToUserPath does nothing when the directory is already on PATH', () => {
  const calls = [];
  const run = (cmd, args) => {
    calls.push(args[0]);
    if (args[0] === 'query') return '    Path    REG_EXPAND_SZ    C:\\other;C:\\tools\\rtdd\n';
    return '';
  };
  assert.equal(addToUserPath('C:\\tools\\rtdd', run), false);
  assert.ok(!calls.includes('add'), 'PATH was rewritten even though the directory was present');
});

// Windows compares paths case-insensitively, so a differently-cased copy is the same entry.
// Without this, every re-run of the installer would append another copy.
test('addToUserPath treats a differently-cased entry as already present', () => {
  const run = (cmd, args) =>
    args[0] === 'query' ? '    Path    REG_EXPAND_SZ    c:\\tools\\RTDD\\\n' : '';
  assert.equal(addToUserPath('C:\\tools\\rtdd', run), false);
});

test('addToUserPath appends when the directory is absent', () => {
  let written = null;
  const run = (cmd, args) => {
    if (args[0] === 'query') return '    Path    REG_EXPAND_SZ    C:\\other\n';
    written = args;
    return '';
  };
  assert.equal(addToUserPath('C:\\tools\\rtdd', run), true);
  assert.equal(written[written.indexOf('/d') + 1], 'C:\\other;C:\\tools\\rtdd');
  // REG_EXPAND_SZ, or the %USERPROFILE% an existing PATH usually contains stops expanding.
  assert.ok(written.includes('REG_EXPAND_SZ'));
});

// A brand new Windows account has no HKCU\Environment\Path at all and `reg query` exits
// non-zero for it. That is an empty PATH, not a failure to install.
test('readUserPath treats a missing registry value as an empty PATH', () => {
  const run = () => {
    throw new Error('ERROR: The system was unable to find the specified registry key');
  };
  assert.equal(readUserPath(run), '');
});

// ---------------------------------------------------------------- parseArgs

test('parseArgs reads flags and environment, with flags winning', () => {
  assert.equal(parseArgs(['--version=v2'], { RTDD_VERSION: 'v1' }).version, 'v2');
  assert.equal(parseArgs([], { RTDD_VERSION: 'v1' }).version, 'v1');
  assert.equal(parseArgs(['--no-skill']).skill, false);
  assert.equal(parseArgs([], { RTDD_NO_SKILL: '1' }).skill, false);
  assert.equal(parseArgs([]).skill, true);
});

test('parseArgs rejects an unknown option instead of ignoring it', () => {
  assert.throws(() => parseArgs(['--yolo']), /unknown option --yolo/);
});

// ---------------------------------------------------------------- end to end

test('install downloads, verifies and places the binary', async (t) => {
  const { goos, goarch } = target();
  const tag = 'v0.0.0-test';
  const archive = archiveName(tag, goos, goarch);
  const bytes = tarball(binaryName(goos), '#!/bin/sh\necho installed\n');
  const server = await serveRelease({ tag, archive, bytes, checksums: `${sha256(bytes)}  ${archive}\n` });
  t.after(() => server.close());

  const installDir = tempDir('rtdd-install-');
  const dest = await install({
    argv: ['--no-skill', `--version=${tag}`, `--install-dir=${installDir}`],
    env: { RTDD_BASE_URL: server.url },
  });

  assert.equal(dest, path.join(installDir, binaryName(goos)));
  assert.equal(fs.readFileSync(dest, 'utf8'), '#!/bin/sh\necho installed\n');
  if (goos !== 'windows') {
    assert.ok(fs.statSync(dest).mode & 0o111, 'the installed binary is not executable');
  }
});

test('install resolves the latest release when no tag is pinned', async (t) => {
  const { goos, goarch } = target();
  const tag = 'v1.4.2';
  const archive = archiveName(tag, goos, goarch);
  const bytes = tarball(binaryName(goos), 'latest');
  const server = await serveRelease({
    tag,
    archive,
    bytes,
    checksums: `${sha256(bytes)}  ${archive}\n`,
    apiBody: JSON.stringify({ tag_name: tag }),
  });
  t.after(() => server.close());

  const installDir = tempDir('rtdd-install-');
  const dest = await install({
    argv: ['--no-skill', `--install-dir=${installDir}`],
    env: { RTDD_BASE_URL: server.url, RTDD_API_URL: `${server.url}/releases/latest` },
  });
  assert.equal(fs.readFileSync(dest, 'utf8'), 'latest');
});

test('install aborts and writes nothing when the checksum does not match', async (t) => {
  const { goos, goarch } = target();
  const tag = 'v0.0.0-test';
  const archive = archiveName(tag, goos, goarch);
  const bytes = tarball(binaryName(goos), 'payload');
  const server = await serveRelease({
    tag,
    archive,
    bytes,
    checksums: `${'0'.repeat(64)}  ${archive}\n`,
  });
  t.after(() => server.close());

  const installDir = tempDir('rtdd-install-');
  await assert.rejects(
    install({
      argv: ['--no-skill', `--version=${tag}`, `--install-dir=${installDir}`],
      env: { RTDD_BASE_URL: server.url },
    }),
    /checksum mismatch/
  );
  assert.equal(fs.readdirSync(installDir).length, 0, 'a binary was written despite a bad checksum');
});

// npm installs a bin as a SYMLINK in node_modules/.bin, so `npx github:VocanicZ/rtdd` runs
// the entrypoint through a path that is not its real one. A main-module check comparing
// process.argv[1] to import.meta.url without resolving symlinks is false there, and the
// installer exits silently having done nothing — which is exactly how it ships to every
// user, since npx is the only documented way to run it.
test('the entrypoint runs when invoked through a symlink, as npx invokes it', async () => {
  const { execFileSync } = await import('node:child_process');
  const dir = tempDir('rtdd-bin-');
  const link = path.join(dir, 'rtdd');
  fs.symlinkSync(path.resolve('installer/index.js'), link);

  const out = execFileSync(process.execPath, [link, '--help'], { encoding: 'utf8' });
  assert.match(out, /npx github:VocanicZ\/rtdd/, 'the entrypoint produced no output through a symlink');
});

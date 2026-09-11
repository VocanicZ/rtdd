// Minimal .tar.gz reader, enough to pull one named member out of a GoReleaser archive.
//
// This is implemented here rather than shelled out to `tar` so the installer depends on
// nothing but Node. `tar` is not a given: Windows only started shipping bsdtar in Windows 10
// 1803, and an installer whose one job is "work on any OS" should not fail on the OS whose
// tooling is least predictable.
import zlib from 'node:zlib';

const BLOCK = 512;

// readString reads a NUL-terminated field. tar pads with NULs, and GNU tar sometimes pads
// with spaces instead, so both are stripped.
function readString(buf, offset, length) {
  const slice = buf.subarray(offset, offset + length);
  let end = slice.indexOf(0);
  if (end === -1) end = slice.length;
  return slice.toString('utf8', 0, end).replace(/\0+$/, '').trim();
}

// readOctal reads a numeric tar field. They are octal ASCII, and an empty field means zero.
function readOctal(buf, offset, length) {
  const text = readString(buf, offset, length);
  if (text === '') return 0;
  const n = parseInt(text, 8);
  return Number.isNaN(n) ? 0 : n;
}

// isZeroBlock reports whether a block is the all-NUL end-of-archive marker.
function isZeroBlock(buf) {
  for (const byte of buf) {
    if (byte !== 0) return false;
  }
  return true;
}

/**
 * listEntries walks an uncompressed tar buffer and yields every regular file it contains.
 *
 * Only regular files are returned: a GoReleaser archive is flat, and directory, symlink and
 * PAX-header entries carry no payload anyone here wants. A PAX or GNU long-name header is
 * skipped along with its data, which is why the name check in extractFile is exact rather
 * than a suffix match — a long-name record would otherwise be mistaken for its own target.
 *
 * @param {Buffer} tar
 * @returns {{name: string, data: Buffer}[]}
 */
export function listEntries(tar) {
  const entries = [];
  let offset = 0;

  while (offset + BLOCK <= tar.length) {
    const header = tar.subarray(offset, offset + BLOCK);
    if (isZeroBlock(header)) break;

    const name = readString(header, 0, 100);
    const size = readOctal(header, 124, 12);
    const typeflag = readString(header, 156, 1) || '0';
    const prefix = readString(header, 345, 155);

    offset += BLOCK;
    const dataStart = offset;
    // Entry data is padded out to a whole number of blocks.
    offset += Math.ceil(size / BLOCK) * BLOCK;

    // '0' and '\0' are both "regular file"; everything else (directories, links, PAX and
    // GNU extension headers) is skipped, its data already stepped over above.
    if (typeflag === '0' || typeflag === '') {
      entries.push({
        name: prefix ? `${prefix}/${name}` : name,
        data: tar.subarray(dataStart, dataStart + size),
      });
    }
  }

  return entries;
}

/**
 * extractFile gunzips a .tar.gz and returns the contents of one member by exact name.
 *
 * @param {Buffer} gzipped raw .tar.gz bytes
 * @param {string} wanted  member name, e.g. "rtdd" or "rtdd.exe"
 * @returns {Buffer}
 * @throws if the archive does not contain that member
 */
export function extractFile(gzipped, wanted) {
  const tar = zlib.gunzipSync(gzipped);
  for (const entry of listEntries(tar)) {
    if (entry.name === wanted) return Buffer.from(entry.data);
  }
  const found = listEntries(tar).map((e) => e.name);
  throw new Error(`archive does not contain ${wanted} (it holds: ${found.join(', ') || 'nothing'})`);
}

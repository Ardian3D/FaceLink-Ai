/**
 * Synthetic JPEG fixture for the staging enrollment harness.
 * Project-generated with FFmpeg 9.0.1 lavfi testsrc2 (112x112, one frame, -q:v 6).
 * JPEG: 3334 bytes; SHA-256 957096ec49bafff4915ed1d33250222a918c95a2ad9869985dfb63a3e74d19a9.
 * Distinct from the pinned positive mannequin fixture; no license claim added.
 *
 * This is a generated test pattern, not a photograph of a person. It is never
 * rendered, logged, persisted, written to the gallery, or sent anywhere except
 * the explicitly selected synthetic enrollment endpoint. The byte array is
 * created only when the simulated capture completes and is dropped after the
 * request finishes.
 *
 * Kept as a compact constant because the staging app must work offline; no
 * asset loader or file is involved.
 */
const FIXTURE_BASE64 = `
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
`;

/** Decode a base64 string without Buffer, atob, or logging its content. */
export function decodeBase64(value: string): Uint8Array {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  const bersih = value.replace(/\s+/g, "").replace(/=+$/, "");
  if (bersih.length % 4 === 1) throw new Error("fixture base64 tidak valid");
  const keluaran = new Uint8Array(Math.floor((bersih.length * 3) / 4));
  let buffer = 0;
  let bit = 0;
  let posisi = 0;
  for (const karakter of bersih) {
    const nilai = alphabet.indexOf(karakter);
    if (nilai < 0) throw new Error("fixture base64 tidak valid");
    buffer = (buffer << 6) | nilai;
    bit += 6;
    if (bit >= 8) {
      bit -= 8;
      keluaran[posisi++] = (buffer >> bit) & 0xff;
    }
  }
  return keluaran;
}

/** Fresh synthetic JPEG bytes; caller owns and must drop the reference. */
export function syntheticJPEG(): Uint8Array {
  return decodeBase64(FIXTURE_BASE64);
}

/**
 * Transport HTTP bersama untuk Go Backend FaceLink AI.
 *
 * Sebelumnya setiap pemeriksaan diagnostik menyusun `fetch`, `AbortController`,
 * dan pembacaan badan sendiri. Autentikasi menambah empat endpoint lagi, dan
 * menyalin pola itu empat kali akan menghasilkan empat perilaku batas waktu
 * yang berbeda. Modul ini menetapkan satu jalur: satu batas waktu, satu cara
 * membaca badan sebagai teks, dan satu bentuk hasil yang dapat dibedakan.
 *
 * Yang **tidak** dilakukan modul ini:
 *  - tidak menafsirkan kode galat backend (itu tugas pemanggil),
 *  - tidak menyimpan atau mencatat token,
 *  - tidak pernah mematikan verifikasi TLS.
 *
 * Badan respons selalu dibaca sebagai **teks**, bukan `json()`: proksi di depan
 * backend dapat menjawab HTML, dan galat "Unexpected token <" tidak menjelaskan
 * apa pun saat debugging.
 */

/** Batas tunggu bawaan satu permintaan. */
export const TIMEOUT_BAWAAN_MS = 5000;

/** Jawaban mentah satu permintaan, sebelum ditafsirkan. */
export type JawabanHttp =
  | { jenis: "jawaban"; status: number; teks: string; durasiMs: number; retryAfter?: string | null; serverDate?: string | null; etag?: string | null }
  | { jenis: "jaringan"; pesan: string; durasiMs: number }
  | { jenis: "timeout"; pesan: string; durasiMs: number };

export type OpsiPermintaan = {
  /** Alamat dasar backend, tanpa garis miring di akhir. */
  baseUrl: string;
  method?: "GET" | "POST" | "DELETE";
  /** Badan JSON. `undefined` berarti permintaan tanpa badan. */
  body?: unknown;
  /** Badan biner mentah (mis. JPEG). Tidak pernah dicatat atau disalin. */
  bodyBytes?: Uint8Array;
  /** Content-Type untuk badan biner; diabaikan untuk JSON. */
  contentType?: string;
  /** Header tambahan yang sudah ditentukan pemanggil (mis. If-Match). */
  headers?: Record<string, string>;
  /** Token akses Bearer. Tidak pernah dimasukkan ke URL atau log. */
  token?: string;
  timeoutMs?: number;
  signal?: AbortSignal;
};

/**
 * Menjalankan satu permintaan HTTP dan mengembalikan hasil yang sudah
 * diklasifikasikan. Tidak pernah melempar: seluruh kegagalan transport menjadi
 * nilai, sehingga pemanggil tidak perlu membungkus setiap panggilan.
 */
export async function mintaHttp(
  path: string,
  opsi: OpsiPermintaan
): Promise<JawabanHttp> {
  const batas = opsi.timeoutMs ?? TIMEOUT_BAWAAN_MS;
  const mulai = Date.now();
  const kendali = new AbortController();
  let habisWaktu = false;
  const batalkan = () => kendali.abort();
  opsi.signal?.addEventListener("abort", batalkan);
  if (opsi.signal?.aborted) kendali.abort();
  const timer = setTimeout(() => {
    habisWaktu = true;
    kendali.abort();
  }, batas);

  // Hanya tajuk yang memang diperlukan. `Accept` menyatakan bahwa jawaban
  // seharusnya JSON; backend tetap dapat menjawab teks galat.
  const tajuk: Record<string, string> = { Accept: "application/json" };
  if (opsi.token !== undefined && opsi.token !== "") {
    tajuk.Authorization = `Bearer ${opsi.token}`;
  }
  if (opsi.body !== undefined) {
    tajuk["Content-Type"] = "application/json";
  }
  if (opsi.bodyBytes !== undefined) {
    tajuk["Content-Type"] = opsi.contentType ?? "application/octet-stream";
  }
  if (opsi.headers !== undefined) {
    Object.assign(tajuk, opsi.headers);
  }

  const badan: BodyInit | undefined =
    opsi.bodyBytes !== undefined
      ? (opsi.bodyBytes as unknown as BodyInit)
      : opsi.body === undefined
        ? undefined
        : JSON.stringify(opsi.body);

  try {
    const respons = await fetch(`${opsi.baseUrl}${path}`, {
      method: opsi.method ?? "GET",
      headers: tajuk,
      body: badan,
      signal: kendali.signal,
    });
    const teks = await respons.text();
    return {
      jenis: "jawaban",
      status: respons.status,
      teks,
      durasiMs: Date.now() - mulai,
      retryAfter: respons.headers.get("Retry-After"),
      serverDate: respons.headers.get("Date"),
      etag: respons.headers.get("ETag"),
    };
  } catch (galat) {
    const durasiMs = Date.now() - mulai;
    if (habisWaktu) {
      return {
        jenis: "timeout",
        pesan: `tidak ada jawaban dalam ${batas} ms`,
        durasiMs,
      };
    }
    return {
      jenis: "jaringan",
      pesan: galat instanceof Error ? galat.message : String(galat),
      durasiMs,
    };
  } finally {
    clearTimeout(timer);
    opsi.signal?.removeEventListener("abort", batalkan);
  }
}

/** `true` bila injeksi JSON yang sudah di-`JSON.parse` berupa objek biasa. */
export function adalahObjek(nilai: unknown): nilai is Record<string, unknown> {
  return typeof nilai === "object" && nilai !== null && !Array.isArray(nilai);
}

/** Membaca badan teks sebagai JSON; `null` bila bukan JSON. */
export function bacaJson(teks: string): unknown {
  if (teks.trim() === "") return null;
  try {
    return JSON.parse(teks);
  } catch {
    return null;
  }
}

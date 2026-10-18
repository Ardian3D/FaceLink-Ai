import { konfigurasiApi } from "./config";

/**
 * Klien HTTP tipis untuk Go Backend FaceLink AI (FL-NET01).
 *
 * Lingkupnya sengaja kecil: dua endpoint kesehatan yang **benar-benar ada**
 * pada `backend/internal/httpapi/router.go` — `GET /api/v1/healthz` dan
 * `GET /api/v1/readyz`. Tidak ada endpoint lain yang dikarang di sini, dan
 * tidak ada token, data peserta, atau apa pun yang bersifat pribadi yang
 * dikirim: pemeriksaan ini hanya membaca status layanan.
 *
 * Tiga hasil dibedakan secara struktural, bukan hanya dari teks pesan, karena
 * ketiganya menuntut tindakan yang berbeda saat debugging:
 *
 *  - **tidak terjangkau** — permintaan tidak pernah sampai (salah alamat,
 *    backend mati, `adb reverse` belum dipasang). Periksa jaringan/port.
 *  - **jawaban galat** — backend menjawab, tetapi status atau bentuknya bukan
 *    yang diharapkan. Periksa log backend, bukan jaringan.
 *  - **sukses** — badan respons terbaca dan bentuknya sesuai kontrak.
 *
 * Kesabaran menunggu dibatasi `TIMEOUT_MS`. Tanpa batas, backend yang mati di
 * tengah koneksi akan menggantung layar diagnostik tanpa penjelasan.
 */

/** Batas tunggu satu permintaan. Cukup longgar untuk laptop lambat, cukup ketat untuk debugging. */
const TIMEOUT_MS = 5000;

/**
 * Hasil satu pemeriksaan, sudah disiapkan untuk ditampilkan.
 *
 * `terjangkau` menjawab satu pertanyaan saja: apakah ada jawaban HTTP dari
 * backend. Ia **tidak** berarti backend sehat — `readyz` yang menjawab `503`
 * tetap `terjangkau: true` dengan `nada: "peringatan"`.
 */
export type HasilCek = {
  terjangkau: boolean;
  nada: "ok" | "peringatan" | "gagal";
  judul: string;
  baris: string[];
  durasiMs: number;
};

export type Diagnostik = {
  baseUrl: string;
  sumber: "environment" | "bawaan-development";
  health: HasilCek;
  ready: HasilCek;
  /** Waktu pemeriksaan selesai, ISO-8601 UTC. */
  pada: string;
};

/** Jawaban mentah satu permintaan, sebelum ditafsirkan. */
type JawabanMentah =
  | { jenis: "jawaban"; status: number; teks: string; durasiMs: number }
  | { jenis: "jaringan"; pesan: string; durasiMs: number }
  | { jenis: "timeout"; pesan: string; durasiMs: number };

/**
 * Type guard untuk JSON yang sudah di-`JSON.parse`.
 *
 * Dipakai alih-alih cast: setiap pembacaan properti sesudahnya tetap diperiksa
 * oleh TypeScript, sehingga backend yang mengubah bentuk respons menghasilkan
 * "bentuk tidak dikenal" — bukan nilai yang diam-diam salah.
 */
function adalahObjek(nilai: unknown): nilai is Record<string, unknown> {
  return typeof nilai === "object" && nilai !== null && !Array.isArray(nilai);
}

async function kirim(path: string, baseUrl: string): Promise<JawabanMentah> {
  const mulai = Date.now();
  const kendali = new AbortController();
  let habisWaktu = false;
  const timer = setTimeout(() => {
    habisWaktu = true;
    kendali.abort();
  }, TIMEOUT_MS);

  try {
    const respons = await fetch(`${baseUrl}${path}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      signal: kendali.signal,
    });
    // Badan dibaca sebagai teks, bukan `json()`: backend dapat menjawab galat
    // HTML dari proksi di depan, dan pesan "Unexpected token <" tidak berguna.
    const teks = await respons.text();
    return { jenis: "jawaban", status: respons.status, teks, durasiMs: Date.now() - mulai };
  } catch (galat) {
    const durasiMs = Date.now() - mulai;
    if (habisWaktu) {
      return { jenis: "timeout", pesan: `tidak ada jawaban dalam ${TIMEOUT_MS} ms`, durasiMs };
    }
    return {
      jenis: "jaringan",
      pesan: galat instanceof Error ? galat.message : String(galat),
      durasiMs,
    };
  } finally {
    clearTimeout(timer);
  }
}

/** Menerjemahkan kegagalan transport menjadi satu hasil yang dapat ditampilkan. */
function hasilTransportGagal(jawaban: Extract<JawabanMentah, { jenis: "jaringan" | "timeout" }>): HasilCek {
  const waktuHabis = jawaban.jenis === "timeout";
  return {
    terjangkau: false,
    nada: "gagal",
    judul: waktuHabis ? "Tidak ada jawaban (batas waktu)" : "Backend tidak terjangkau",
    baris: [
      jawaban.pesan,
      "Periksa: backend berjalan? port benar? `adb reverse tcp:8080 tcp:8080` sudah dipasang?",
    ],
    durasiMs: jawaban.durasiMs,
  };
}

/** Memotong badan respons yang tidak dikenal agar tetap muat di layar. */
function ringkasTeks(teks: string): string {
  const rapi = teks.trim().replace(/\s+/g, " ");
  return rapi.length <= 200 ? rapi : `${rapi.slice(0, 200)}…`;
}

/** Nilai string dari objek, atau tanda pisah bila tidak ada. */
function teksAtau(objek: Record<string, unknown>, kunci: string, cadangan = "—"): string {
  const nilai = objek[kunci];
  return typeof nilai === "string" ? nilai : cadangan;
}

/**
 * `GET /api/v1/healthz` — liveness.
 *
 * Kontrak `docs/API_CONTRACT.md` §5.12: badan sukses hanya `status`, `version`,
 * `time`; tidak ada jalur berkas, versi pustaka, atau status perangkat. Klien
 * memeriksa bentuk itu, sehingga perubahan tak terduga pada backend terlihat
 * sebagai "bentuk tidak dikenal", bukan sebagai sukses palsu.
 */
async function cekHealth(baseUrl: string): Promise<HasilCek> {
  const jawaban = await kirim("/api/v1/healthz", baseUrl);
  if (jawaban.jenis !== "jawaban") return hasilTransportGagal(jawaban);

  if (jawaban.status !== 200) {
    return {
      terjangkau: true,
      nada: "gagal",
      judul: `healthz menjawab ${jawaban.status}`,
      baris: [ringkasTeks(jawaban.teks)],
      durasiMs: jawaban.durasiMs,
    };
  }

  let isi: unknown;
  try {
    isi = JSON.parse(jawaban.teks);
  } catch {
    return {
      terjangkau: true,
      nada: "gagal",
      judul: "healthz menjawab 200 tetapi badannya bukan JSON",
      baris: [ringkasTeks(jawaban.teks)],
      durasiMs: jawaban.durasiMs,
    };
  }

  if (!adalahObjek(isi) || typeof isi.status !== "string") {
    return {
      terjangkau: true,
      nada: "gagal",
      judul: "Bentuk respons healthz tidak dikenal",
      baris: ["Diharapkan objek dengan `status` bertipe string.", ringkasTeks(jawaban.teks)],
      durasiMs: jawaban.durasiMs,
    };
  }

  return {
    terjangkau: true,
    nada: isi.status === "ok" ? "ok" : "peringatan",
    judul: `healthz: ${isi.status}`,
    baris: [`version: ${teksAtau(isi, "version")}`, `waktu server: ${teksAtau(isi, "time")}`],
    durasiMs: jawaban.durasiMs,
  };
}

/**
 * `GET /api/v1/readyz` — kesiapan.
 *
 * Dua bentuk badan ditangani karena keduanya sah:
 *  - `200` → `{status, database, ai_engine, migrations, wal}`
 *  - `503` → `{code, message, request_id, details:{...}}`
 *
 * `ai_engine: "unavailable"` **tidak** digagalkan di sini. Sesuai
 * `docs/SECURITY_PRIVACY.md` §5e, mesin AI yang belum siap tidak menggagalkan
 * absensi manual, dan backend tetap menjawab `200`. Yang dilaporkan adalah
 * kenyataannya, bukan kesimpulan yang lebih keras daripada backend sendiri.
 */
async function cekReady(baseUrl: string): Promise<HasilCek> {
  const jawaban = await kirim("/api/v1/readyz", baseUrl);
  if (jawaban.jenis !== "jawaban") return hasilTransportGagal(jawaban);

  let isi: unknown;
  try {
    isi = JSON.parse(jawaban.teks);
  } catch {
    return {
      terjangkau: true,
      nada: "gagal",
      judul: `readyz menjawab ${jawaban.status} tetapi badannya bukan JSON`,
      baris: [ringkasTeks(jawaban.teks)],
      durasiMs: jawaban.durasiMs,
    };
  }

  if (!adalahObjek(isi)) {
    return {
      terjangkau: true,
      nada: "gagal",
      judul: "Bentuk respons readyz tidak dikenal",
      baris: [ringkasTeks(jawaban.teks)],
      durasiMs: jawaban.durasiMs,
    };
  }

  // Saat 503, rincian komponen berada di `details`; saat 200, di akar objek.
  const rincian = adalahObjek(isi.details) ? isi.details : isi;
  const aiEngine = teksAtau(rincian, "ai_engine");
  const wal = adalahObjek(rincian.wal) ? rincian.wal : null;
  const baris = [
    `database: ${teksAtau(rincian, "database")}`,
    `migrasi: ${teksAtau(rincian, "migrations")}`,
    `WAL: ${wal === null ? "—" : teksAtau(wal, "journal_mode")}`,
    // Sengaja eksplisit: mesin AI belum tentu ada, dan ketiadaannya bukan
    // kegagalan backend.
    `mesin AI: ${aiEngine}${aiEngine === "unavailable" ? " (tidak menghalangi backend)" : ""}`,
  ];

  if (jawaban.status === 200) {
    const status = teksAtau(isi, "status");
    return {
      terjangkau: true,
      nada: status === "ready" ? "ok" : "peringatan",
      judul: `readyz: ${status}`,
      baris,
      durasiMs: jawaban.durasiMs,
    };
  }

  // Backend menjawab, jadi ia hidup — tetapi menyatakan dirinya belum siap.
  const kode = teksAtau(isi, "code");
  const pesan = teksAtau(isi, "message", "");
  return {
    terjangkau: true,
    nada: "peringatan",
    judul: `readyz: ${jawaban.status} ${kode}`,
    baris: pesan === "" ? baris : [pesan, ...baris],
    durasiMs: jawaban.durasiMs,
  };
}

/**
 * Menjalankan kedua pemeriksaan. Dipanggil dari layar diagnostik development
 * dan tidak pernah dari alur produksi.
 */
export async function jalankanDiagnostik(): Promise<Diagnostik | { galat: string }> {
  if (!konfigurasiApi.ok) {
    return { galat: konfigurasiApi.alasan };
  }

  const baseUrl = konfigurasiApi.baseUrl;
  // Berurutan, bukan paralel: bila alamatnya salah, pengguna melihat satu
  // kegagalan yang jelas lebih dahulu, bukan dua kegagalan identik sekaligus.
  const health = await cekHealth(baseUrl);
  const ready = await cekReady(baseUrl);

  return {
    baseUrl,
    sumber: konfigurasiApi.sumber,
    health,
    ready,
    pada: new Date().toISOString(),
  };
}

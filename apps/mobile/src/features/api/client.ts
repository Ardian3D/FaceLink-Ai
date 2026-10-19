import { konfigurasiApi } from "./config";
import { adalahObjek, bacaJson, mintaHttp, type JawabanHttp } from "./http";

/**
 * Klien diagnostik Go Backend FaceLink AI (FL-NET01).
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
 * Kesabaran menunggu dibatasi `TIMEOUT_BAWAAN_MS` pada `./http`: satu nilai
 * untuk seluruh permintaan, sehingga diagnostik dan autentikasi tidak
 * berperilaku berbeda saat backend menggantung.
 */

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



/**
 * Menerjemahkan kegagalan transport menjadi satu hasil yang dapat ditampilkan.
 *
 * Tipe hasil diambil dari transport bersama, sehingga penambahan jenis
 * kegagalan baru di sana memaksa fungsi ini diperbarui alih-alih terlewat.
 */
function hasilTransportGagal(
  jawaban: Extract<JawabanHttp, { jenis: "jaringan" | "timeout" }>
): HasilCek {
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
  const jawaban = await mintaHttp("/api/v1/healthz", { baseUrl });
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
    isi = bacaJson(jawaban.teks);
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
  const jawaban = await mintaHttp("/api/v1/readyz", { baseUrl });
  if (jawaban.jenis !== "jawaban") return hasilTransportGagal(jawaban);

  let isi: unknown;
  try {
    isi = bacaJson(jawaban.teks);
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

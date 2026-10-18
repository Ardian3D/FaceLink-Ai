/**
 * Konfigurasi alamat Go Backend untuk aplikasi mobile.
 *
 * **Tidak ada alamat LAN, IP laptop, atau rahasia yang ditanam di sini.**
 * Alamat datang dari variabel lingkungan Expo `EXPO_PUBLIC_FACELINK_API_BASE_URL`
 * yang di-*inline* saat bundel dibuat (lihat `.env.example` dan
 * `apps/mobile/README.md`). Mengubahnya tidak memerlukan perubahan kode.
 *
 * Satu-satunya nilai bawaan adalah **loopback** `http://127.0.0.1:8080`, dan
 * hanya pada development build. Nilai itu bukan alamat LAN: ia berguna justru
 * karena perangkat Android tidak dapat menjangkau loopback laptop kecuali lewat
 * `adb reverse tcp:8080 tcp:8080`, yang memetakan port perangkat ke port laptop.
 * Pada build produksi (`__DEV__ === false`) tidak ada nilai bawaan sama sekali,
 * sehingga alamat wajib diberikan lewat environment.
 *
 * Aturan transport yang ditegakkan di sini, bukan hanya didokumentasikan:
 *
 *  - `https:` selalu diterima.
 *  - `http:` hanya diterima untuk host loopback atau `10.0.2.2` (alias host dari
 *    emulator Android). Ini menutup kemungkinan "menyelesaikan" masalah koneksi
 *    dengan mengirim token lewat HTTP ke alamat LAN, yang dilarang D-035.
 *  - Alamat dengan kredensial (`user:pass@host`), query, atau fragmen ditolak.
 *
 * Validasi TLS **tidak pernah** dimatikan oleh modul ini: tidak ada opsi
 * `InsecureSkipVerify` atau ekuivalennya pada `fetch` React Native, dan tidak
 * boleh ditambahkan.
 */

/**
 * Nilai bawaan development: loopback, bukan alamat LAN.
 *
 * Dipakai hanya bila `EXPO_PUBLIC_FACELINK_API_BASE_URL` tidak diisi **dan**
 * build adalah development. Angka 8080 mengikuti `config.DefaultAddr` pada
 * `backend/internal/config/config.go`.
 */
const BAWAAN_DEV = "http://127.0.0.1:8080";

/** Host yang boleh memakai HTTP polos: loopback dan alias emulator Android. */
const HOST_HTTP_DIIZINKAN: Record<string, true> = {
  "127.0.0.1": true,
  localhost: true,
  "::1": true,
  "[::1]": true,
  "10.0.2.2": true,
};

export type HasilKonfigurasi =
  | { ok: true; baseUrl: string; sumber: "environment" | "bawaan-development" }
  | { ok: false; alasan: string };

/**
 * Memeriksa satu nilai alamat. Dipisah dari pembacaan environment supaya dapat
 * diuji tanpa mengubah `process.env`.
 */
export function periksaBaseUrl(nilai: string): HasilKonfigurasi {
  const bersih = nilai.trim().replace(/\/+$/, "");
  if (bersih === "") {
    return { ok: false, alasan: "alamat kosong" };
  }

  let url: URL;
  try {
    url = new URL(bersih);
  } catch {
    return { ok: false, alasan: "bukan URL yang sah (contoh yang benar: http://127.0.0.1:8080)" };
  }

  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return { ok: false, alasan: `skema ${url.protocol} tidak didukung; pakai http atau https` };
  }

  if (url.username !== "" || url.password !== "") {
    return { ok: false, alasan: "alamat tidak boleh memuat kredensial" };
  }

  if (url.search !== "" || url.hash !== "") {
    return { ok: false, alasan: "alamat tidak boleh memuat query atau fragmen" };
  }

  if (url.protocol === "http:" && HOST_HTTP_DIIZINKAN[url.hostname] !== true) {
    // Bukan sekadar kerapian: token sesi akan dikirim ke alamat ini pada
    // integrasi berikutnya, dan HTTP polos di jaringan lokal dilarang D-035.
    return {
      ok: false,
      alasan: `HTTP polos hanya diizinkan untuk loopback atau emulator (10.0.2.2); ${url.hostname} menuntut https`,
    };
  }

  return { ok: true, baseUrl: bersih, sumber: "environment" };
}

/**
 * Alamat yang dipakai aplikasi. Dibaca sekali saat modul dimuat karena
 * `EXPO_PUBLIC_*` sudah berupa konstanta pada bundel.
 */
export const konfigurasiApi: HasilKonfigurasi = (() => {
  const dariEnv = process.env.EXPO_PUBLIC_FACELINK_API_BASE_URL;

  if (dariEnv !== undefined && dariEnv.trim() !== "") {
    return periksaBaseUrl(dariEnv);
  }

  if (__DEV__) {
    const bawaan = periksaBaseUrl(BAWAAN_DEV);
    return bawaan.ok ? { ...bawaan, sumber: "bawaan-development" } : bawaan;
  }

  return {
    ok: false,
    alasan:
      "EXPO_PUBLIC_FACELINK_API_BASE_URL belum disetel pada build ini; alamat backend wajib diberikan saat membangun",
  };
})();

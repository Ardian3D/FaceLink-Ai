/**
 * Konfigurasi Google Sign-In untuk aplikasi mobile.
 *
 * Ada **satu** nilai yang harus disediakan build: **Web OAuth client ID** dari
 * Google Cloud project yang sama dengan `FACELINK_GOOGLE_CLIENT_IDS` pada
 * backend. Nilai itu bukan rahasia (ia memang dikirim sebagai `aud` pada ID
 * token), tetapi ia **tidak** ditanam di dalam kode: build yang berbeda dapat
 * memakai project Google yang berbeda, dan menanamkannya akan membuat build
 * lokal dan build demo saling mengunci.
 *
 * Aturan bentuk yang ditegakkan di sini:
 *  - harus berakhiran `.apps.googleusercontent.com` — nilai lain hampir pasti
 *    client ID Android/iOS, yang **tidak dapat** dipakai sebagai `aud` ID token
 *    untuk backend;
 *  - tidak boleh kosong.
 *
 * Bila nilai tidak ada, aplikasi **tidak** menampilkan tombol masuk yang
 * seolah berfungsi: alasan konfigurasi dilaporkan apa adanya, karena
 * memperbaikinya menuntut build ulang, bukan percobaan ulang di HP.
 */

export type HasilKonfigurasiGoogle =
  | { ok: true; webClientId: string }
  | { ok: false; alasan: string };

/** Memeriksa satu nilai client ID. Dipisah agar dapat diuji tanpa environment. */
export function periksaWebClientId(nilai: string): HasilKonfigurasiGoogle {
  const bersih = nilai.trim();
  if (bersih === "") {
    return { ok: false, alasan: "client ID Google kosong" };
  }
  if (!bersih.endsWith(".apps.googleusercontent.com")) {
    return {
      ok: false,
      alasan:
        "client ID harus bertipe Web (berakhiran .apps.googleusercontent.com); client ID Android/iOS tidak dapat dipakai sebagai audience backend",
    };
  }
  return { ok: true, webClientId: bersih };
}

/**
 * Nilai yang berlaku untuk build ini. Dibaca sekali saat modul dimuat karena
 * `EXPO_PUBLIC_*` sudah berupa konstanta pada bundel.
 */
export const konfigurasiGoogle: HasilKonfigurasiGoogle = (() => {
  const dariEnv = process.env.EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID;
  if (dariEnv === undefined) {
    return {
      ok: false,
      alasan:
        "EXPO_PUBLIC_FACELINK_GOOGLE_WEB_CLIENT_ID belum disetel pada build ini; masuk dengan Google memerlukan Web OAuth client ID",
    };
  }
  return periksaWebClientId(dariEnv);
})();

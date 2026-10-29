/**
 * Penyimpanan token sesi di perangkat.
 *
 * **Aturan yang ditegakkan modul ini (D-033, `docs/SECURITY_PRIVACY.md`):**
 * token akses dan token penyegar hanya boleh berada di penyimpanan kredensial
 * platform — `expo-secure-store` (Android Keystore / iOS Keychain) — dan
 * **tidak pernah** di `AsyncStorage`. Google ID token **tidak** disimpan sama
 * sekali: ia hanya hidup selama satu pertukaran di memori.
 *
 * Cache memori di depan penyimpanan aman ada karena dua alasan nyata:
 *  1. Setiap pembacaan `SecureStore` adalah operasi kriptografis; membacanya
 *     pada setiap permintaan HTTP akan memperlambat dashboard tanpa manfaat.
 *  2. Penulisan ganda saat rotasi token tidak perlu menyentuh disk dua kali.
 *
 * Cache itu **tidak** memperpanjang umur token: nilai yang dikembalikan selalu
 * nilai yang terakhir tersimpan, dan `hapusSesiTersimpan()` mengosongkan
 * keduanya. Bila penyimpanan aman gagal (mis. Keystore sedang terkunci), modul
 * ini mengembalikan `null`/melempar galat yang jelas — ia tidak pernah jatuh ke
 * penyimpanan tidak aman.
 */
import * as SecureStore from "expo-secure-store";

/**
 * Nama kunci penyimpanan. Nilainya sengaja tidak memuat istilah yang berguna
 * bagi penyerang yang membaca nama kunci (tidak ada "refresh_token" mentah),
 * tetapi tetap stabil karena harus dapat dibaca setelah aplikasi dimulai ulang.
 */
const KUNCI = "facelink.session.v1";

/**
 * Sesi yang tersimpan: pasangan token mentah dan waktu kedaluwarsa akses.
 *
 * `expiresAtMs` memakai jam perangkat dan hanya dipakai untuk memutuskan kapan
 * menyegarkan lebih awal. Keputusan akhir tentang keabsahan tetap milik
 * backend; jam perangkat yang salah tidak boleh membuat aplikasi menganggap
 * sesi sah.
 */
export type SesiTersimpan = {
  accessToken: string;
  refreshToken: string;
  /** Waktu kedaluwarsa token akses dalam milidetik epoch. */
  expiresAtMs: number;
};

let cache: SesiTersimpan | null = null;
let sudahDimuat = false;
let tersedia: boolean | null = null;
let generasiMutasi = 0;
/**
 * Apakah penyimpanan kredensial platform ada pada build ini.
 *
 * `expo-secure-store` hanya berfungsi di Android dan iOS; pada bundel web
 * modulnya kosong, sehingga memanggilnya langsung akan melempar
 * `TypeError: ... is not a function` saat aplikasi dibuka. Pemeriksaan ini
 * membuat pratinjau web berjalan normal tanpa sesi tersimpan, bukan berhenti
 * pada layar pemulihan. Tidak ada fallback ke penyimpanan tidak aman.
 */
async function penyimpananTersedia(): Promise<boolean> {
  if (tersedia !== null) return tersedia;
  tersedia = await SecureStore.isAvailableAsync().catch(() => false);
  return tersedia;
}

/** `true` bila token akses masih berlaku minimal selama `marginMs`. */
export function aksesMasihBerlaku(
  sesi: SesiTersimpan,
  marginMs = 60_000
): boolean {
  return sesi.expiresAtMs - marginMs > Date.now();
}

/**
 * Membaca sesi tersimpan. Hasil `null` berarti belum pernah masuk atau sesi
 * sudah dihapus; ia bukan keadaan galat.
 *
 * Isi mentah diperiksa bentuknya sebelum dipakai: penyimpanan dapat berisi
 * data dari versi aplikasi lain atau sisa penulisan yang terpotong, dan
 * mempercayainya begitu saja akan mengirim `undefined` sebagai Bearer.
 */
export async function muatSesiTersimpan(): Promise<SesiTersimpan | null> {
  if (sudahDimuat) return cache;
  const generasi = generasiMutasi;
  sudahDimuat = true;
  if (!(await penyimpananTersedia())) return null;
  const mentah = await SecureStore.getItemAsync(KUNCI);
  if (generasi !== generasiMutasi) return cache;
  if (mentah === null) {
    cache = null;
    return null;
  }
  const sesi = bacaSesi(mentah);
  cache = sesi;
  if (sesi === null) {
    // Data rusak dibuang, bukan dibiarkan: menyimpannya hanya akan mengulang
    // kegagalan yang sama pada setiap peluncuran berikutnya.
    await SecureStore.deleteItemAsync(KUNCI);
  }
  return sesi;
}

/**
 * Menyimpan sesi. Galat penyimpanan dilempar agar pemanggil dapat menampilkannya.
 *
 * Pada platform tanpa penyimpanan kredensial (web) fungsi ini melempar, bukan
 * diam-diam menyimpan di tempat lain: menyimpan token di penyimpanan biasa
 * melanggar D-033, dan sesi yang tampak berhasil padahal tidak tersimpan hanya
 * akan menghilang tanpa penjelasan saat aplikasi dibuka ulang.
 */
export async function simpanSesi(sesi: SesiTersimpan): Promise<void> {
  if (!(await penyimpananTersedia())) {
    throw new Error("penyimpanan kredensial platform tidak tersedia");
  }
  const generasi = ++generasiMutasi;
  await SecureStore.setItemAsync(KUNCI, JSON.stringify(sesi), {
    // Token hanya perlu tersedia setelah perangkat dibuka kuncinya; tidak ada
    // alasan menyalinnya ke perangkat lain melalui pencadangan.
    keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
  });
  if (generasi === generasiMutasi) {
    cache = sesi;
    sudahDimuat = true;
  }
}

/**
 * Menghapus sesi dari cache dan penyimpanan aman.
 *
 * Dipanggil saat keluar, saat backend menyatakan sesi dicabut, dan saat token
 * penyegar ditolak. Penghapusan dilakukan lebih dahulu, baru keadaan UI
 * diubah: bila proses berhenti di tengah, aplikasi tidak boleh bangun dengan
 * token yang seharusnya sudah dibuang.
 */
export async function hapusSesiTersimpan(): Promise<void> {
  generasiMutasi += 1;
  cache = null;
  sudahDimuat = true;
  if (!(await penyimpananTersedia())) return;
  await SecureStore.deleteItemAsync(KUNCI);
}

function bacaSesi(mentah: string): SesiTersimpan | null {
  let nilai: unknown;
  try {
    nilai = JSON.parse(mentah);
  } catch {
    return null;
  }
  if (typeof nilai !== "object" || nilai === null) return null;
  const kandidat = nilai as Record<string, unknown>;
  const { accessToken, refreshToken, expiresAtMs } = kandidat;
  if (
    typeof accessToken !== "string" ||
    accessToken === "" ||
    typeof refreshToken !== "string" ||
    refreshToken === "" ||
    typeof expiresAtMs !== "number" ||
    !Number.isFinite(expiresAtMs)
  ) {
    return null;
  }
  return { accessToken, refreshToken, expiresAtMs };
}

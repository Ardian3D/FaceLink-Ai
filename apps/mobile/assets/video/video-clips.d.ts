import type { VideoSource } from "expo-video";

/**
 * Tipe untuk modul virtual `../../assets/video/video-clips`.
 *
 * Implementasinya dipilih Metro saat bundel dibuat: `video-clips.local.js` bila
 * klip video ada di folder ini, atau `video-clips.none.js` bila tidak (lihat
 * `metro.config.js`). Kode aplikasi tidak pernah mengimpor keduanya langsung.
 */
export declare const localVideoClips: VideoSource[];

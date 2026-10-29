import { enrollmentStagingAktif } from "./staging";

export type CaptureMode = "SYNTHETIC_FLOW" | "MANNEQUIN_PREVIEW_ONLY";

/**
 * Pure guard shared by tests and the screen.
 *
 * MANNEQUIN mode is impossible outside staging and is never enabled by the
 * default staging environment. It must be requested explicitly at build time.
 */
export function resolveCaptureMode(staging: boolean, requested: string | undefined): CaptureMode {
  if (!staging) return "SYNTHETIC_FLOW";
  const value = (requested ?? "").trim().toLowerCase();
  return value === "mannequin_preview_only" || value === "mannequin_preview"
    ? "MANNEQUIN_PREVIEW_ONLY"
    : "SYNTHETIC_FLOW";
}

export const captureMode = resolveCaptureMode(
  enrollmentStagingAktif,
  process.env.EXPO_PUBLIC_FACELINK_CAPTURE_MODE
);

export const captureModeLabel =
  captureMode === "MANNEQUIN_PREVIEW_ONLY"
    ? "MANNEQUIN · TANPA UPLOAD"
    : "SINTETIS · TANPA KAMERA";

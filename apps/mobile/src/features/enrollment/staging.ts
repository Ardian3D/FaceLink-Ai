/**
 * Explicit staging switch. The production/main build leaves this false.
 * The enrollment route itself is generated only into the staging app, so a
 * main build cannot reach the synthetic workflow even if a deep link is tried.
 */
export const enrollmentStagingAktif =
  process.env.EXPO_PUBLIC_FACELINK_ENR_STAGING === "1";

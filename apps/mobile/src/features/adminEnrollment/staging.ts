import { enrollmentStagingAktif } from "@/features/enrollment/staging";

/** Route is generated only into the disposable synthetic staging tree. */
export const adminEnrollmentStagingAktif =
  enrollmentStagingAktif && process.env.EXPO_PUBLIC_FACELINK_ADMIN_ENR_STAGING === "1";

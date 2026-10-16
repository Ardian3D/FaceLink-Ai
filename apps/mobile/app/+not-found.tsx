import React from "react";
import { useRouter } from "expo-router";

import { AppButton } from "@/components/AppButton";
import { Screen } from "@/components/Screen";
import { StateView } from "@/components/StateView";

/** Rute tidak dikenal: diarahkan kembali ke layar masuk, bukan layar kosong. */
export default function NotFound() {
  const router = useRouter();

  return (
    <Screen center>
      <StateView
        tone="empty"
        title="Halaman tidak ditemukan"
        description="Tautan yang Anda buka tidak ada di aplikasi ini."
        action={<AppButton label="Kembali ke layar masuk" onPress={() => router.replace("/")} />}
      />
    </Screen>
  );
}

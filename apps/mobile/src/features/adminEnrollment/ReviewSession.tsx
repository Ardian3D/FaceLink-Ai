import React, { createContext, useContext, useEffect, useState } from "react";
import { useSession } from "@/features/auth/SessionProvider";
import { adminApi } from "./api";
import { AdminReviewFlow, type AdminReviewState } from "./flow";

type ReviewSession = { acquire: () => AdminReviewFlow; dispose: () => void };
const Context = createContext<{ owner: ReviewSession; state: AdminReviewState | null } | null>(null);

/** Authenticated-layout lifetime, not route lifetime. No persistence or bearer in React state. */
export function AdminReviewProvider({ children }: { children: React.ReactNode }) {
  const { getAccessToken } = useSession();
  const [state, setState] = useState<AdminReviewState | null>(null);
  // The existing getter closes over the stable AuthFlow, not a token snapshot.
  // Lazy acquisition also permits React StrictMode's effect cleanup/setup cycle.
  const [owner] = useState<ReviewSession>(() => {
    let controller: AdminReviewFlow | null = null;
    return {
      acquire: () => controller ??= new AdminReviewFlow(adminApi, getAccessToken, setState),
      dispose: () => { controller?.dispose(); controller = null; },
    };
  });
  useEffect(() => () => owner.dispose(), [owner]);
  return <Context.Provider value={{ owner, state }}>{children}</Context.Provider>;
}
export function useAdminReviewSession() {
  const value = useContext(Context);
  if (!value) throw new Error("Admin review requires its authenticated staging provider");
  return value;
}

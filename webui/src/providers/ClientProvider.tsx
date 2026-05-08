import { createContext, useContext, type ReactNode } from "react";

import type { FlyflorClient } from "@/lib/flyflor-client";

interface ClientContextValue {
  client: FlyflorClient;
  token: string;
  modelName: string | null;
  blackboardMode: string | null;
}

const ClientContext = createContext<ClientContextValue | null>(null);

export function ClientProvider({
  client,
  token,
  modelName = null,
  blackboardMode = null,
  children,
}: {
  client: FlyflorClient;
  token: string;
  modelName?: string | null;
  blackboardMode?: string | null;
  children: ReactNode;
}) {
  return (
    <ClientContext.Provider value={{ client, token, modelName, blackboardMode }}>
      {children}
    </ClientContext.Provider>
  );
}

export function useClient(): ClientContextValue {
  const ctx = useContext(ClientContext);
  if (!ctx) {
    throw new Error("useClient must be used within a ClientProvider");
  }
  return ctx;
}

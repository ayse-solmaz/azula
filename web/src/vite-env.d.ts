/// <reference types="vite/client" />

interface AzulaDesktop {
  shell?: boolean;
  graphqlUrl?: string | (() => string);
  deviceId?: string;
  deviceName?: string;
  getToken?: () => string;
  hasSession?: () => boolean;
  setSession?: (token: string | null) => void;
}

interface Window {
  azulaDesktop?: AzulaDesktop;
}

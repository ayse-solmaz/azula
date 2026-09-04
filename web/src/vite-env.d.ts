/// <reference types="vite/client" />

interface AzulaDesktop {
  shell?: boolean;
  graphqlUrl?: string;
  deviceId?: string;
  deviceName?: string;
}

interface Window {
  azulaDesktop?: AzulaDesktop;
}

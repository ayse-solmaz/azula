import { createRoot } from "react-dom/client";
import { BrowserRouter, HashRouter } from "react-router-dom";
import App from "./App";
import { applyChrome, I18nProvider } from "./i18n";
import "./styles.css";

applyChrome();

const Router = window.azulaDesktop || window.location.protocol === "file:" ? HashRouter : BrowserRouter;

createRoot(document.getElementById("root")!).render(
  <Router>
    <I18nProvider>
      <App />
    </I18nProvider>
  </Router>
);

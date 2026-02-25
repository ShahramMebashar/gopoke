import { registerExtension } from "@codingame/monaco-vscode-api/extensions";

// Built-in themes — import whenReady so we can await full loading
import { whenReady as defaultThemesReady } from "@codingame/monaco-vscode-theme-defaults-default-extension";
import { whenReady as monokaiReady } from "@codingame/monaco-vscode-theme-monokai-default-extension";
import { whenReady as solarizedDarkReady } from "@codingame/monaco-vscode-theme-solarized-dark-default-extension";
import { whenReady as solarizedLightReady } from "@codingame/monaco-vscode-theme-solarized-light-default-extension";

// Custom third-party themes registered as a single extension
const themes = [
  { id: "GitHub Dark Default", label: "GitHub Dark", uiTheme: "vs-dark", path: "./themes/github-dark.json" },
  { id: "GitHub Light Default", label: "GitHub Light", uiTheme: "vs", path: "./themes/github-light.json" },
  { id: "One Dark Pro", label: "One Dark Pro", uiTheme: "vs-dark", path: "./themes/one-dark-pro.json" },
  { id: "Dracula", label: "Dracula", uiTheme: "vs-dark", path: "./themes/dracula.json" },
  { id: "Material Theme Darker", label: "Material Darker", uiTheme: "vs-dark", path: "./themes/material-darker.json" },
  { id: "Nord", label: "Nord", uiTheme: "vs-dark", path: "./themes/nord.json" },
  { id: "Catppuccin Mocha", label: "Catppuccin Mocha", uiTheme: "vs-dark", path: "./themes/catppuccin-mocha.json" },
];

const { registerFileUrl, whenReady: customThemesReady } = registerExtension({
  name: "gopad-themes",
  displayName: "Gopad Custom Themes",
  version: "1.0.0",
  engines: { vscode: "*" },
  contributes: { themes },
});

for (const theme of themes) {
  registerFileUrl(
    theme.path,
    new URL(`./themes/${theme.path.split("/").pop()}`, import.meta.url).toString(),
    { mimeType: "application/json" },
  );
}

// Single promise that resolves when ALL theme extensions have fully loaded their JSON files
export const allThemesReady = Promise.all([
  defaultThemesReady(),
  monokaiReady(),
  solarizedDarkReady(),
  solarizedLightReady(),
  customThemesReady(),
]);

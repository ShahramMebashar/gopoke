import React, { useMemo, useRef, useCallback, useState, useEffect } from "react";
import * as vscode from "vscode";
import { MonacoEditorReactComp } from "@typefox/monaco-editor-react";
import { configureDefaultWorkerFactory } from "monaco-languageclient/workerFactory";
import { updateUserConfiguration } from "@codingame/monaco-vscode-configuration-service-override";

// Side-effect imports: register VS Code theme + Go language extensions
import "@codingame/monaco-vscode-go-default-extension";
import { allThemesReady } from "./themes.js";

// Full config including theme — used after services + themes are ready
function buildConfigJson(theme, fontSize, fontFamily, lineNumbers) {
  return JSON.stringify({
    "workbench.colorTheme": theme,
    "editor.fontSize": fontSize,
    "editor.fontFamily": fontFamily,
    "editor.lineNumbers": lineNumbers,
    "editor.minimap.enabled": false,
    "editor.wordBasedSuggestions": "off",
    "editor.lightbulb.enabled": "On",
  });
}

// Bootstrap config WITHOUT theme — theme is applied later via updateUserConfiguration
// so the config service detects a genuine file change when we add the theme key.
function buildBootstrapConfigJson(fontSize, fontFamily, lineNumbers) {
  return JSON.stringify({
    "editor.fontSize": fontSize,
    "editor.fontFamily": fontFamily,
    "editor.lineNumbers": lineNumbers,
    "editor.minimap.enabled": false,
    "editor.wordBasedSuggestions": "off",
    "editor.lightbulb.enabled": "On",
  });
}

export default function GopadMonacoEditor({
  code,
  onCodeChange,
  wsPort,
  workspaceDir,
  theme = "Default Dark Modern",
  fontSize = 14,
  fontFamily = "monospace",
  lineNumbers = "on",
  onEditorReady,
}) {
  const [editorReady, setEditorReady] = useState(false);
  const [themesReady, setThemesReady] = useState(false);

  // Track when all theme JSONs have been fetched
  useEffect(() => {
    let cancelled = false;
    allThemesReady.then(() => { if (!cancelled) setThemesReady(true); });
    return () => { cancelled = true; };
  }, []);

  // One-time bootstrap config — consumed once by apiWrapper.start()
  const vscodeApiConfig = useMemo(
    () => ({
      $type: "extended",
      viewsConfig: { $type: "EditorService" },
      advanced: { loadThemes: false },
      userConfiguration: {
        json: buildBootstrapConfigJson(fontSize, fontFamily, lineNumbers),
      },
      monacoWorkerFactory: configureDefaultWorkerFactory,
    }),
    // Intentionally static — runtime changes go through updateUserConfiguration
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  // Push config to the live VS Code configuration service.
  // Fires when: editor becomes ready, themes finish loading, or any setting prop changes.
  // Both editorReady AND themesReady must be true (services must exist + theme JSONs fetched).
  useEffect(() => {
    if (!editorReady || !themesReady) return;
    updateUserConfiguration(buildConfigJson(theme, fontSize, fontFamily, lineNumbers));
  }, [editorReady, themesReady, theme, fontSize, fontFamily, lineNumbers]);

  const editorAppConfig = useMemo(
    () => ({
      codeResources: {
        modified: {
          text: code,
          uri: `file://${workspaceDir}/main.go`,
        },
      },
    }),
    // Only use workspaceDir for URI — code synced via onTextChanged
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [workspaceDir]
  );

  const languageClientConfig = useMemo(() => {
    if (!wsPort) return undefined;
    return {
      languageId: "go",
      connection: {
        options: {
          $type: "WebSocketUrl",
          url: `ws://localhost:${wsPort}/lsp`,
        },
      },
      clientOptions: {
        documentSelector: ["go"],
        workspaceFolder: {
          index: 0,
          name: "workspace",
          uri: vscode.Uri.file(workspaceDir || "/tmp"),
        },
      },
    };
  }, [wsPort, workspaceDir]);

  const handleTextChanged = useCallback(
    (textChanges) => {
      if (textChanges.modified != null && onCodeChange) {
        onCodeChange(textChanges.modified);
      }
    },
    [onCodeChange]
  );

  const handleEditorStartDone = useCallback(
    (app) => {
      setEditorReady(true);
      if (onEditorReady) {
        onEditorReady(app);
      }
    },
    [onEditorReady]
  );

  return (
    <MonacoEditorReactComp
      style={{ height: "100%", width: "100%" }}
      vscodeApiConfig={vscodeApiConfig}
      editorAppConfig={editorAppConfig}
      languageClientConfig={languageClientConfig}
      onTextChanged={handleTextChanged}
      onEditorStartDone={handleEditorStartDone}
      onError={(e) => console.error("Monaco editor error:", e)}
    />
  );
}

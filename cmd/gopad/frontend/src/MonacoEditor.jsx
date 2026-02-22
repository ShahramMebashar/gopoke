import React, { useMemo, useRef, useCallback, useState, useEffect } from "react";
import * as vscode from "vscode";
import { MonacoEditorReactComp } from "@typefox/monaco-editor-react";
import { configureDefaultWorkerFactory } from "monaco-languageclient/workerFactory";
import { updateUserConfiguration } from "@codingame/monaco-vscode-configuration-service-override";

// Side-effect imports: register VS Code theme + Go language extensions
import { whenReady as goExtensionReady } from "@codingame/monaco-vscode-go-default-extension";
import { allThemesReady } from "./themes.js";

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

const allExtensionsReady = Promise.allSettled([allThemesReady, goExtensionReady()]);

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
  const [extensionsReady, setExtensionsReady] = useState(false);
  const [editorReady, setEditorReady] = useState(false);
  const editorAppRef = useRef(null);
  const startupNudgeDoneRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    allExtensionsReady
      .then((results) => {
        for (const result of results) {
          if (result.status === "rejected") {
            console.error("Monaco extension readiness failed", result.reason);
          }
        }
        if (!cancelled) {
          setExtensionsReady(true);
        }
      })
      .catch((error) => {
        console.error("Monaco extension readiness failed", error);
        if (!cancelled) {
          setExtensionsReady(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const refreshEditorPresentation = useCallback(() => {
    const editor = editorAppRef.current?.getEditor?.();
    const model = editor?.getModel?.();
    if (!editor || !model) return;

    try {
      if (typeof model.resetTokenization === "function") {
        model.resetTokenization();
      }
    } catch (error) {
      console.error("Monaco resetTokenization failed", error);
    }

    try {
      if (typeof model.forceTokenization === "function") {
        const lines =
          typeof model.getLineCount === "function" ? model.getLineCount() : 1;
        model.forceTokenization(lines);
      }
    } catch (error) {
      console.error("Monaco forceTokenization failed", error);
    }

    try {
      const languageId =
        typeof model.getLanguageId === "function" ? model.getLanguageId() : "";
      if (typeof model.setLanguage === "function" && languageId) {
        model.setLanguage("plaintext");
        model.setLanguage(languageId);
      }
    } catch (error) {
      console.error("Monaco language toggle failed", error);
    }

    try {
      if (typeof editor.render === "function") {
        editor.render(true);
      }
      if (typeof editor.layout === "function") {
        editor.layout();
      }
    } catch (error) {
      console.error("Monaco repaint failed", error);
    }

    if (!startupNudgeDoneRef.current) {
      startupNudgeDoneRef.current = true;
      try {
        if (
          typeof model.getValue === "function" &&
          typeof model.setValue === "function"
        ) {
          const current = model.getValue();
          model.setValue(current);
        }
      } catch (error) {
        console.error("Monaco startup nudge failed", error);
      }
    }
  }, []);

  const vscodeApiConfig = useMemo(
    () => ({
      $type: "extended",
      viewsConfig: { $type: "EditorService" },
      userConfiguration: {
        json: buildConfigJson(theme, fontSize, fontFamily, lineNumbers),
      },
      monacoWorkerFactory: configureDefaultWorkerFactory,
    }),
    [theme, fontSize, fontFamily, lineNumbers],
  );

  useEffect(() => {
    if (!editorReady) return;
    const json = buildConfigJson(theme, fontSize, fontFamily, lineNumbers);
    void updateUserConfiguration(json)
      .then(() => {
        // Nudge Monaco to re-read theme/token colors without requiring user edits.
        requestAnimationFrame(() => refreshEditorPresentation());
      })
      .catch((error) => {
        // Keep editor usable even if configuration write fails.
        console.error("failed to apply Monaco user configuration", error);
      });
  }, [
    editorReady,
    extensionsReady,
    theme,
    fontSize,
    fontFamily,
    lineNumbers,
    refreshEditorPresentation,
  ]);

  const editorAppConfig = useMemo(
    () => ({
      codeResources: {
        modified: {
          text: code,
          uri: `file://${workspaceDir}/main.go`,
        },
      },
    }),
    [code, workspaceDir],
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
    [onCodeChange],
  );

  const handleEditorStartDone = useCallback(
    (app) => {
      editorAppRef.current = app;
      setEditorReady(true);
      requestAnimationFrame(() => refreshEditorPresentation());
      if (onEditorReady) onEditorReady(app);
    },
    [onEditorReady, refreshEditorPresentation],
  );

  return (
    <MonacoEditorReactComp
      style={{ height: "100%", width: "100%" }}
      vscodeApiConfig={vscodeApiConfig}
      editorAppConfig={editorAppConfig}
      languageClientConfig={languageClientConfig}
      onTextChanged={handleTextChanged}
      onEditorStartDone={handleEditorStartDone}
      onError={(e) => {
        console.error("Monaco error", e);
      }}
    />
  );
}

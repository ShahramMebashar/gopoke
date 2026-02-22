import React, { useMemo, useRef, useCallback } from "react";
import * as vscode from "vscode";
import { MonacoEditorReactComp } from "@typefox/monaco-editor-react";
import { configureDefaultWorkerFactory } from "monaco-languageclient/workerFactory";

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
  const reprocessRef = useRef(0);

  const vscodeApiConfig = useMemo(
    () => ({
      $type: "extended",
      viewsConfig: { $type: "EditorService" },
      userConfiguration: {
        json: JSON.stringify({
          "workbench.colorTheme": theme,
          "editor.fontSize": fontSize,
          "editor.fontFamily": fontFamily,
          "editor.lineNumbers": lineNumbers,
          "editor.minimap.enabled": false,
          "editor.wordBasedSuggestions": "off",
          "editor.lightbulb.enabled": "On",
        }),
      },
      monacoWorkerFactory: configureDefaultWorkerFactory,
    }),
    [theme, fontSize, fontFamily, lineNumbers]
  );

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
      if (onEditorReady) {
        onEditorReady(app);
      }
    },
    [onEditorReady]
  );

  // Bump reprocess counter when config props change
  const reprocessCounter = useMemo(() => {
    reprocessRef.current += 1;
    return reprocessRef.current;
  }, [vscodeApiConfig]);

  return (
    <MonacoEditorReactComp
      style={{ height: "100%", width: "100%" }}
      vscodeApiConfig={vscodeApiConfig}
      editorAppConfig={editorAppConfig}
      languageClientConfig={languageClientConfig}
      onTextChanged={handleTextChanged}
      onEditorStartDone={handleEditorStartDone}
      onError={(e) => console.error("Monaco editor error:", e)}
      triggerReprocessConfig={reprocessCounter}
    />
  );
}

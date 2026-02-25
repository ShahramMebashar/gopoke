import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import GopokeMonacoEditor from "./MonacoEditor";
import RichOutput from "./renderers/RichOutput";
import SettingsPanel from "./SettingsPanel";
import Toolbar from "./Toolbar";
import {
  availableToolchains,
  cancelRun,
  chooseGoFile,
  chooseProjectDirectory,
  deleteProjectEnvVar,
  deleteProjectSnippet,
  formatSnippet,
  openGoFile,
  playgroundShare,
  playgroundImport,
  lspStatus,
  lspWebSocketPort,
  lspWorkspaceInfo,
  onRunStderrChunk,
  onRunStdoutChunk,
  openProject,
  projectEnvVars,
  projectSnippets,
  recentProjects,
  runSnippet,
  saveGoFile,
  saveProjectSnippet,
  setProjectDefaultPackage,
  setProjectToolchain,
  setProjectWorkingDirectory,
  upsertProjectEnvVar,
} from "./wailsBridge";

const defaultEditorSettings = {
  theme: "Default Dark Modern",
  fontFamily: "JetBrains Mono",
  fontSize: 14,
  lineNumbers: true,
};

function loadEditorSettings() {
  try {
    const raw = localStorage.getItem("gopoke:editor-settings");
    if (raw) return { ...defaultEditorSettings, ...JSON.parse(raw) };
  } catch {}
  return defaultEditorSettings;
}

function saveEditorSettings(settings) {
  localStorage.setItem("gopoke:editor-settings", JSON.stringify(settings));
}

const defaultSnippet = [
  "package main",
  "",
  'import "fmt"',
  "",
  "func main() {",
  '\tfmt.Println("Starting report...")',
  '\tfmt.Println(`//gopoke:table [{"endpoint":"/api/users","latency_ms":12},{"endpoint":"/api/orders","latency_ms":45}]`)',
  '\tfmt.Println(`//gopoke:json {"total_requests":1520,"avg_latency_ms":28.5,"status":"healthy"}`)',
  '\tfmt.Println("Done.")',
  "}",
].join("\n");

function normalizeError(error) {
  if (!error) return "Unexpected error.";
  if (typeof error === "string") return error;
  if (error.message) return error.message;
  return JSON.stringify(error);
}

function formatDateTime(value) {
  if (!value) return "Unknown time";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "Unknown time";
  return parsed.toLocaleString();
}

function normalizeRunStdoutChunk(payload) {
  if (!payload || typeof payload !== "object") return null;
  const runId =
    typeof payload.runId === "string"
      ? payload.runId
      : typeof payload.RunID === "string"
        ? payload.RunID
        : "";
  const chunk =
    typeof payload.chunk === "string"
      ? payload.chunk
      : typeof payload.Chunk === "string"
        ? payload.Chunk
        : "";
  if (!runId || chunk.length === 0) return null;
  return { runId, chunk };
}

function normalizeRunStderrChunk(payload) {
  if (!payload || typeof payload !== "object") return null;
  const runId =
    typeof payload.runId === "string"
      ? payload.runId
      : typeof payload.RunID === "string"
        ? payload.RunID
        : "";
  const chunk =
    typeof payload.chunk === "string"
      ? payload.chunk
      : typeof payload.Chunk === "string"
        ? payload.Chunk
        : "";
  if (!runId || chunk.length === 0) return null;
  return { runId, chunk };
}

function emptyRunResult() {
  return {
    Stdout: "",
    Stderr: "",
    ExitCode: null,
    DurationMS: null,
    TimedOut: false,
    Canceled: false,
    StdoutTruncated: false,
    StderrTruncated: false,
    Diagnostics: [],
    CleanStdout: "",
    RichBlocks: [],
  };
}

function formatDurationMs(value) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    return "N/A";
  }
  return `${value}ms`;
}

function formatExitCode(value) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "N/A";
  }
  return String(value);
}

function runStateLabel(runState) {
  switch (runState) {
    case "running":
      return "Running";
    case "success":
      return "Success";
    case "failed":
      return "Failed";
    case "canceled":
      return "Canceled";
    default:
      return "Idle";
  }
}

function normalizeDiagnostics(items) {
  if (!Array.isArray(items)) return [];
  return items
    .map((item) => {
      if (!item || typeof item !== "object") return null;
      const kind =
        typeof item.Kind === "string"
          ? item.Kind
          : typeof item.kind === "string"
            ? item.kind
            : "";
      const file =
        typeof item.File === "string"
          ? item.File
          : typeof item.file === "string"
            ? item.file
            : "";
      const message =
        typeof item.Message === "string"
          ? item.Message
          : typeof item.message === "string"
            ? item.message
            : "";
      const lineRaw =
        typeof item.Line === "number"
          ? item.Line
          : typeof item.line === "number"
            ? item.line
            : 0;
      const columnRaw =
        typeof item.Column === "number"
          ? item.Column
          : typeof item.column === "number"
            ? item.column
            : 0;
      const line = Number.isFinite(lineRaw) ? Math.max(0, Math.floor(lineRaw)) : 0;
      const column = Number.isFinite(columnRaw)
        ? Math.max(0, Math.floor(columnRaw))
        : 0;
      if (!message && line <= 0) return null;
      return {
        kind: kind || "unknown",
        file,
        line,
        column,
        message: message || "Unknown diagnostic",
      };
    })
    .filter(Boolean);
}

function diagnosticTitle(item) {
  const position =
    item.line > 0
      ? `${item.file || "snippet"}:${item.line}${item.column > 0 ? `:${item.column}` : ""}`
      : item.file || "snippet";
  return `${position} - ${item.message}`;
}

function normalizeEnvVar(item) {
  if (!item || typeof item !== "object") return null;
  const key =
    typeof item.Key === "string"
      ? item.Key
      : typeof item.key === "string"
        ? item.key
        : "";
  if (!key) return null;
  return {
    ID:
      typeof item.ID === "string"
        ? item.ID
        : typeof item.id === "string"
          ? item.id
          : "",
    Key: key,
    Value:
      typeof item.Value === "string"
        ? item.Value
        : typeof item.value === "string"
          ? item.value
          : "",
    Masked: item.Masked === true || item.masked === true,
  };
}

function normalizeToolchain(item) {
  if (!item || typeof item !== "object") return null;
  const path =
    typeof item.Path === "string"
      ? item.Path
      : typeof item.path === "string"
        ? item.path
        : "";
  if (!path) return null;
  return {
    Name:
      typeof item.Name === "string"
        ? item.Name
        : typeof item.name === "string"
          ? item.name
          : path,
    Path: path,
    Version:
      typeof item.Version === "string"
        ? item.Version
        : typeof item.version === "string"
          ? item.version
          : "unknown",
  };
}

function normalizeSnippetRecord(item) {
  if (!item || typeof item !== "object") return null;
  const id =
    typeof item.ID === "string"
      ? item.ID
      : typeof item.id === "string"
        ? item.id
        : "";
  const name =
    typeof item.Name === "string"
      ? item.Name
      : typeof item.name === "string"
        ? item.name
        : "";
  if (!id || !name) return null;
  return {
    ID: id,
    Name: name,
    Content:
      typeof item.Content === "string"
        ? item.Content
        : typeof item.content === "string"
          ? item.content
          : "",
    UpdatedAt:
      typeof item.UpdatedAt === "string"
        ? item.UpdatedAt
        : typeof item.updatedAt === "string"
          ? item.updatedAt
          : "",
  };
}

function chooseCopyName(name, existing) {
  const base = (name || "Snippet").trim() || "Snippet";
  const taken = new Set(existing.map((item) => item.Name.toLowerCase()));
  const plainCopy = `${base} Copy`;
  if (!taken.has(plainCopy.toLowerCase())) {
    return plainCopy;
  }
  for (let i = 2; i < 1000; i += 1) {
    const candidate = `${base} Copy ${i}`;
    if (!taken.has(candidate.toLowerCase())) {
      return candidate;
    }
  }
  return `${base} Copy`;
}

function normalizeProjectRecord(raw) {
  if (!raw || typeof raw !== "object") return null;
  return {
    ID: raw.ID || raw.id || "",
    Path: raw.Path || raw.path || "",
    LastOpenedAt: raw.LastOpenedAt || raw.lastOpenedAt || "",
    DefaultPkg: raw.DefaultPkg || raw.defaultPackage || "",
    WorkingDir: raw.WorkingDir || raw.workingDirectory || "",
    Toolchain: raw.Toolchain || raw.toolchain || "",
  };
}

function normalizeOpenProjectResult(raw) {
  if (!raw || typeof raw !== "object") return null;
  return {
    Project: normalizeProjectRecord(raw.Project),
    Module: raw.Module || {},
    Targets: Array.isArray(raw.Targets) ? raw.Targets : [],
    EnvVars: Array.isArray(raw.EnvVars) ? raw.EnvVars : [],
    EnvLoadWarnings: Array.isArray(raw.EnvLoadWarnings) ? raw.EnvLoadWarnings : [],
  };
}

function readProjectTargets(result) {
  return Array.isArray(result?.Targets) ? result.Targets : [];
}

function readOpenEnvVars(result) {
  const vars = Array.isArray(result?.EnvVars) ? result.EnvVars : [];
  return vars.map(normalizeEnvVar).filter(Boolean);
}

function readOpenWarnings(result) {
  return Array.isArray(result?.EnvLoadWarnings) ? result.EnvLoadWarnings : [];
}

export default function App() {
  const [status, setStatus] = useState({ kind: "info", message: "Ready." });
  const [isBusy, setIsBusy] = useState(false);
  const [projectPathInput, setProjectPathInput] = useState("");
  const [snippet, setSnippet] = useState(defaultSnippet);
  const [recent, setRecent] = useState([]);
  const [activeProjectResult, setActiveProjectResult] = useState(null);
  const [selectedTarget, setSelectedTarget] = useState("");
  const [runResult, setRunResult] = useState(null);
  const [activeRunId, setActiveRunId] = useState("");
  const [lastRunSource, setLastRunSource] = useState("");
  const [runState, setRunState] = useState("idle");
  const [outputTab, setOutputTab] = useState("raw");

  // Sidebar state
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarTab, setSidebarTab] = useState("project");
  const [diagExpanded, setDiagExpanded] = useState(false);

  const [envVars, setEnvVars] = useState([]);
  const [envKeyInput, setEnvKeyInput] = useState("");
  const [envValueInput, setEnvValueInput] = useState("");
  const [envMaskedInput, setEnvMaskedInput] = useState(false);
  const [envRevealMap, setEnvRevealMap] = useState({});

  const [workingDirectory, setWorkingDirectory] = useState("");
  const [toolchains, setToolchains] = useState([]);
  const [selectedToolchain, setSelectedToolchain] = useState("go");

  const [snippets, setSnippets] = useState([]);
  const [snippetSearch, setSnippetSearch] = useState("");
  const [selectedSnippetId, setSelectedSnippetId] = useState("");
  const [snippetNameInput, setSnippetNameInput] = useState("");

  // Editor appearance settings
  const [editorSettings, setEditorSettings] = useState(loadEditorSettings);
  const [settingsOpen, setSettingsOpen] = useState(false);

  // Import dialog state (replaces window.prompt which doesn't work in WKWebView)
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [importDialogValue, setImportDialogValue] = useState("");

  // Platform detection: default to darwin to prevent HTML toolbar flash on macOS
  const [platform, setPlatform] = useState("darwin");

  const updateEditorSetting = useCallback((key, value) => {
    setEditorSettings((prev) => {
      const next = { ...prev, [key]: value };
      saveEditorSettings(next);
      return next;
    });
  }, []);

  // File import state: tracks the on-disk .go file path when opened via "Open File"
  const [activeFilePath, setActiveFilePath] = useState("");

  // LSP connection state
  const [lspPort, setLspPort] = useState(0);
  const [lspWorkspaceDir, setLspWorkspaceDir] = useState("");

  const activeRunIdRef = useRef("");
  const runHandlerRef = useRef(null);
  const saveFileHandlerRef = useRef(null);
  const editorAppRef = useRef(null);

  // Helper: push content into both Monaco model and React state.
  // Monaco ignores prop updates after mount, so we must call model.setValue() directly.
  const setEditorContent = useCallback((content) => {
    const editor = editorAppRef.current?.getEditor?.();
    const model = editor?.getModel?.();
    if (model) {
      model.setValue(content);
    }
    setSnippet(content);
  }, []);

  const lineCount = useMemo(
    () => (snippet.length === 0 ? 0 : snippet.split("\n").length),
    [snippet],
  );
  const diagnostics = useMemo(
    () => normalizeDiagnostics(runResult ? runResult.Diagnostics : []),
    [runResult],
  );
  const richBlocks = useMemo(() => {
    if (!runResult) return [];
    return Array.isArray(runResult.RichBlocks) ? runResult.RichBlocks : [];
  }, [runResult]);
  const hasRichBlocks = richBlocks.length > 0;
  const combinedOutput = useMemo(() => {
    if (!runResult) return "";
    const stdout = hasRichBlocks && runResult.CleanStdout != null
      ? runResult.CleanStdout
      : runResult.Stdout;
    const parts = [];
    if (stdout) parts.push(stdout);
    if (runResult.Stderr) {
      if (parts.length > 0) parts.push("\n--- stderr ---\n");
      parts.push(runResult.Stderr);
    }
    return parts.join("");
  }, [runResult, hasRichBlocks]);

  const filteredSnippets = useMemo(() => {
    const search = snippetSearch.trim().toLowerCase();
    if (!search) return snippets;
    return snippets.filter((item) => {
      const name = item.Name.toLowerCase();
      const content = item.Content.toLowerCase();
      return name.includes(search) || content.includes(search);
    });
  }, [snippetSearch, snippets]);

  const workingDirectoryOptions = useMemo(() => {
    if (!activeProjectResult || !activeProjectResult.Project) return [];
    const values = new Set();
    if (activeProjectResult.Project.Path) {
      values.add(activeProjectResult.Project.Path);
    }
    for (const target of readProjectTargets(activeProjectResult)) {
      if (target && typeof target.Path === "string" && target.Path) {
        values.add(target.Path);
      }
    }
    if (workingDirectory) {
      values.add(workingDirectory);
    }
    return Array.from(values);
  }, [activeProjectResult, workingDirectory]);

  const setProjectRecordPatch = useCallback((patch) => {
    setActiveProjectResult((current) => {
      if (!current || !current.Project) return current;
      return {
        ...current,
        Project: {
          ...current.Project,
          ...patch,
        },
      };
    });
  }, []);

  const loadRecentProjects = useCallback(async () => {
    try {
      const projects = await recentProjects(12);
      const normalized = Array.isArray(projects)
        ? projects.map(normalizeProjectRecord).filter(Boolean)
        : [];
      setRecent(normalized);
    } catch (error) {
      setStatus({
        kind: "error",
        message: `Failed loading recent projects: ${normalizeError(error)}`,
      });
    }
  }, []);

  const refreshProjectEnv = useCallback(async (projectPath) => {
    const vars = await projectEnvVars(projectPath);
    const normalized = Array.isArray(vars)
      ? vars.map(normalizeEnvVar).filter(Boolean)
      : [];
    setEnvVars(normalized);
  }, []);

  const refreshProjectSnippets = useCallback(async (projectPath) => {
    const loaded = await projectSnippets(projectPath);
    const normalized = Array.isArray(loaded)
      ? loaded.map(normalizeSnippetRecord).filter(Boolean)
      : [];
    setSnippets(normalized);
  }, []);

  const refreshToolchains = useCallback(async (projectToolchain = "") => {
    const loaded = await availableToolchains();
    const normalized = Array.isArray(loaded)
      ? loaded.map(normalizeToolchain).filter(Boolean)
      : [];
    setToolchains(normalized);

    if (projectToolchain && projectToolchain.trim()) {
      setSelectedToolchain(projectToolchain);
      return;
    }
    if (normalized.length > 0) {
      setSelectedToolchain(normalized[0].Path || normalized[0].Name);
    }
  }, []);

  useEffect(() => {
    void loadRecentProjects();
  }, [loadRecentProjects]);

  // Fetch LSP connection info on mount (scratch workspace starts at app boot)
  useEffect(() => {
    const fetchLspInfo = async () => {
      try {
        const port = await lspWebSocketPort();
        const wsInfo = await lspWorkspaceInfo();
        if (port) setLspPort(port);
        if (wsInfo?.dir) setLspWorkspaceDir(wsInfo.dir);
      } catch {}
    };
    fetchLspInfo();
  }, []);

  // Detect platform once on mount (Wails runtime API)
  useEffect(() => {
    const detect = async () => {
      try {
        const env = await window.runtime.Environment();
        if (env?.platform) setPlatform(env.platform);
      } catch {}
    };
    detect();
  }, []);

  useEffect(() => {
    const cancel = onRunStdoutChunk((payload) => {
      const streamChunk = normalizeRunStdoutChunk(payload);
      if (!streamChunk) return;
      if (!activeRunIdRef.current || streamChunk.runId !== activeRunIdRef.current) {
        return;
      }
      setRunResult((current) => {
        const base = current || emptyRunResult();
        return {
          ...base,
          Stdout: `${base.Stdout || ""}${streamChunk.chunk}`,
        };
      });
    });
    return () => cancel();
  }, []);

  useEffect(() => {
    const cancel = onRunStderrChunk((payload) => {
      const streamChunk = normalizeRunStderrChunk(payload);
      if (!streamChunk) return;
      if (!activeRunIdRef.current || streamChunk.runId !== activeRunIdRef.current) {
        return;
      }
      setRunResult((current) => {
        const base = current || emptyRunResult();
        return {
          ...base,
          Stderr: `${base.Stderr || ""}${streamChunk.chunk}`,
        };
      });
    });
    return () => cancel();
  }, []);


  // Keyboard shortcuts: Cmd+B toggle sidebar, Cmd+Enter run, Cmd+1-4 open tabs
  useEffect(() => {
    const handleKeyDown = (event) => {
      const isMod = event.metaKey || event.ctrlKey;
      if (!isMod) return;

      if (event.key === "s" || event.key === "S") {
        event.preventDefault();
        saveFileHandlerRef.current?.();
        return;
      }

      if (event.key === "Enter") {
        event.preventDefault();
        runHandlerRef.current?.();
        return;
      }

      if (event.key === "b" || event.key === "B") {
        event.preventDefault();
        setSidebarOpen((open) => !open);
        return;
      }

      if (event.key === ",") {
        event.preventDefault();
        setSettingsOpen((v) => !v);
        return;
      }

      const tabMap = { "1": "snippets", "2": "env", "3": "project", "4": "recent" };
      if (tabMap[event.key]) {
        event.preventDefault();
        setSidebarOpen(true);
        setSidebarTab(tabMap[event.key]);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const handleOpenProject = useCallback(
    async (path) => {
      const trimmed = path.trim();
      if (!trimmed) {
        setStatus({ kind: "error", message: "Project path is required." });
        return;
      }

      setIsBusy(true);
      try {
        const raw = await openProject(trimmed);
        const result = normalizeOpenProjectResult(raw);
        setActiveProjectResult(result);
        setActiveFilePath("");
        setRunState("idle");
        setRunResult(null);
        const targets = readProjectTargets(result);
        const defaultTarget =
          result?.Project?.DefaultPkg ||
          (targets.length > 0 ? targets[0].Package : "");
        setSelectedTarget(defaultTarget);
        setProjectPathInput(result?.Project?.Path || trimmed);

        const loadedEnv = readOpenEnvVars(result);
        setEnvVars(loadedEnv);
        setEnvRevealMap({});
        setEnvKeyInput("");
        setEnvValueInput("");
        setEnvMaskedInput(false);

        setWorkingDirectory(result?.Project?.WorkingDir || result?.Project?.Path || "");
        setSelectedToolchain(result?.Project?.Toolchain || "go");
        setSelectedSnippetId("");
        setSnippetNameInput("");
        setSnippetSearch("");

        await Promise.all([
          loadRecentProjects(),
          refreshProjectSnippets(result?.Project?.Path || trimmed),
          refreshToolchains(result?.Project?.Toolchain || ""),
        ]);

        // Fetch LSP connection info (LSP started by openProject)
        try {
          const port = await lspWebSocketPort();
          const wsInfo = await lspWorkspaceInfo();
          setLspPort(port || 0);
          setLspWorkspaceDir(wsInfo?.dir || "");
        } catch {}

        const warnings = readOpenWarnings(result);
        if (warnings.length > 0) {
          setStatus({
            kind: "info",
            message: `Opened project with ${warnings.length} .env warning(s).`,
          });
        } else {
          setStatus({
            kind: "success",
            message: `Opened project: ${result.Project.Path}`,
          });
        }
      } catch (error) {
        setActiveProjectResult(null);
        setSelectedTarget("");
        setEnvVars([]);
        setSnippets([]);
        setStatus({ kind: "error", message: normalizeError(error) });
      } finally {
        setIsBusy(false);
      }
    },
    [loadRecentProjects, refreshProjectSnippets, refreshToolchains],
  );

  const handlePickDirectory = useCallback(async () => {
    try {
      const selectedPath = await chooseProjectDirectory();
      if (typeof selectedPath !== "string" || selectedPath.trim().length === 0) {
        setStatus({
          kind: "info",
          message:
            "No folder selected. If the native picker closes unexpectedly, paste a path and use Open Path.",
        });
        return;
      }
      await handleOpenProject(selectedPath);
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    }
  }, [handleOpenProject]);

  const handlePickGoFile = useCallback(async () => {
    try {
      const selectedPath = await chooseGoFile();
      if (typeof selectedPath !== "string" || selectedPath.trim().length === 0) {
        setStatus({ kind: "info", message: "No file selected." });
        return;
      }
      setIsBusy(true);
      const raw = await openGoFile(selectedPath);
      const content = raw?.content || "";
      const filePath = raw?.filePath || selectedPath;
      const projectRaw = raw?.projectResult;
      const result = normalizeOpenProjectResult(projectRaw);

      setEditorContent(content);
      setActiveFilePath(filePath);
      setActiveProjectResult(result);
      setRunState("idle");
      setRunResult(null);

      const targets = readProjectTargets(result);
      const defaultTarget =
        result?.Project?.DefaultPkg ||
        (targets.length > 0 ? targets[0].Package : "");
      setSelectedTarget(defaultTarget);
      setProjectPathInput(result?.Project?.Path || "");

      const loadedEnv = readOpenEnvVars(result);
      setEnvVars(loadedEnv);
      setEnvRevealMap({});
      setEnvKeyInput("");
      setEnvValueInput("");
      setEnvMaskedInput(false);

      setWorkingDirectory(result?.Project?.WorkingDir || result?.Project?.Path || "");
      setSelectedToolchain(result?.Project?.Toolchain || "go");
      setSelectedSnippetId("");
      setSnippetNameInput("");
      setSnippetSearch("");

      await Promise.all([
        loadRecentProjects(),
        refreshProjectSnippets(result?.Project?.Path || ""),
        refreshToolchains(result?.Project?.Toolchain || ""),
      ]);

      try {
        const port = await lspWebSocketPort();
        const wsInfo = await lspWorkspaceInfo();
        setLspPort(port || 0);
        setLspWorkspaceDir(wsInfo?.dir || "");
      } catch {}

      setStatus({ kind: "success", message: `Opened file: ${filePath}` });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [loadRecentProjects, refreshProjectSnippets, refreshToolchains]);

  const handleSaveDefaultTarget = useCallback(async () => {
    if (!activeProjectResult || !activeProjectResult.Project.Path) {
      setStatus({
        kind: "error",
        message: "Open a project before saving a default target.",
      });
      return;
    }
    if (!selectedTarget) {
      setStatus({ kind: "error", message: "Select a valid run target." });
      return;
    }

    setIsBusy(true);
    try {
      const raw = await setProjectDefaultPackage(
        activeProjectResult.Project.Path,
        selectedTarget,
      );
      const updated = normalizeProjectRecord(raw);
      setProjectRecordPatch({ DefaultPkg: updated.DefaultPkg || selectedTarget });
      await loadRecentProjects();
      setStatus({
        kind: "success",
        message: `Saved default target: ${updated.DefaultPkg || selectedTarget}`,
      });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [activeProjectResult, loadRecentProjects, selectedTarget, setProjectRecordPatch]);

  const handleSaveWorkingDirectory = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before updating working directory." });
      return;
    }
    if (!workingDirectory.trim()) {
      setStatus({ kind: "error", message: "Choose a working directory." });
      return;
    }

    setIsBusy(true);
    try {
      const raw = await setProjectWorkingDirectory(
        activeProjectResult.Project.Path,
        workingDirectory,
      );
      const updated = normalizeProjectRecord(raw);
      setProjectRecordPatch({ WorkingDir: updated.WorkingDir || workingDirectory });
      setWorkingDirectory(updated.WorkingDir || workingDirectory);
      setStatus({
        kind: "success",
        message: `Saved working directory: ${updated.WorkingDir || workingDirectory}`,
      });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [activeProjectResult, setProjectRecordPatch, workingDirectory]);

  const handleSaveToolchain = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before selecting a toolchain." });
      return;
    }
    if (!selectedToolchain.trim()) {
      setStatus({ kind: "error", message: "Select a Go toolchain." });
      return;
    }

    setIsBusy(true);
    try {
      const raw = await setProjectToolchain(
        activeProjectResult.Project.Path,
        selectedToolchain,
      );
      const updated = normalizeProjectRecord(raw);
      setProjectRecordPatch({ Toolchain: updated.Toolchain || selectedToolchain });
      setSelectedToolchain(updated.Toolchain || selectedToolchain);
      setStatus({
        kind: "success",
        message: `Saved toolchain: ${updated.Toolchain || selectedToolchain}`,
      });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [activeProjectResult, selectedToolchain, setProjectRecordPatch]);

  const handleSaveEnvVar = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before editing environment variables." });
      return;
    }
    if (!envKeyInput.trim()) {
      setStatus({ kind: "error", message: "Environment key is required." });
      return;
    }

    setIsBusy(true);
    try {
      await upsertProjectEnvVar(
        activeProjectResult.Project.Path,
        envKeyInput.trim(),
        envValueInput,
        envMaskedInput,
      );
      await refreshProjectEnv(activeProjectResult.Project.Path);
      setEnvRevealMap((current) => ({
        ...current,
        [envKeyInput.trim()]: !envMaskedInput,
      }));
      setStatus({ kind: "success", message: `Saved env var: ${envKeyInput.trim()}` });
      setEnvKeyInput("");
      setEnvValueInput("");
      setEnvMaskedInput(false);
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [
    activeProjectResult,
    envKeyInput,
    envMaskedInput,
    envValueInput,
    refreshProjectEnv,
  ]);

  const handleDeleteEnvVar = useCallback(
    async (key) => {
      if (!activeProjectResult?.Project?.Path) return;
      setIsBusy(true);
      try {
        await deleteProjectEnvVar(activeProjectResult.Project.Path, key);
        await refreshProjectEnv(activeProjectResult.Project.Path);
        setEnvRevealMap((current) => {
          const next = { ...current };
          delete next[key];
          return next;
        });
        setStatus({ kind: "success", message: `Deleted env var: ${key}` });
      } catch (error) {
        setStatus({ kind: "error", message: normalizeError(error) });
      } finally {
        setIsBusy(false);
      }
    },
    [activeProjectResult, refreshProjectEnv],
  );

  const handleSelectSnippet = useCallback((record) => {
    if (!record) return;
    setSelectedSnippetId(record.ID);
    setSnippetNameInput(record.Name);
    setEditorContent(record.Content || "");
    setStatus({ kind: "info", message: `Loaded snippet: ${record.Name}` });
  }, [setEditorContent]);

  const handleCreateNewSnippet = useCallback(() => {
    setSelectedSnippetId("");
    setSnippetNameInput("New Snippet");
    setEditorContent(defaultSnippet);
    setStatus({ kind: "info", message: "Preparing a new snippet." });
  }, [setEditorContent]);

  const handleSaveSnippet = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before saving snippets." });
      return;
    }
    if (!snippetNameInput.trim()) {
      setStatus({ kind: "error", message: "Snippet name is required." });
      return;
    }

    setIsBusy(true);
    try {
      const saved = normalizeSnippetRecord(await saveProjectSnippet(
        activeProjectResult.Project.Path,
        selectedSnippetId,
        snippetNameInput.trim(),
        snippet,
      ));
      await refreshProjectSnippets(activeProjectResult.Project.Path);
      setSelectedSnippetId(saved?.ID || selectedSnippetId);
      setSnippetNameInput(saved?.Name || snippetNameInput.trim());
      setStatus({ kind: "success", message: `Saved snippet: ${saved?.Name || snippetNameInput.trim()}` });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [
    activeProjectResult,
    refreshProjectSnippets,
    selectedSnippetId,
    snippet,
    snippetNameInput,
  ]);

  const handleDuplicateSnippet = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before duplicating snippets." });
      return;
    }

    const baseName = snippetNameInput.trim() || "Snippet";
    const duplicateName = chooseCopyName(baseName, snippets);

    setIsBusy(true);
    try {
      const duplicated = normalizeSnippetRecord(await saveProjectSnippet(
        activeProjectResult.Project.Path,
        "",
        duplicateName,
        snippet,
      ));
      await refreshProjectSnippets(activeProjectResult.Project.Path);
      setSelectedSnippetId(duplicated?.ID || "");
      setSnippetNameInput(duplicated?.Name || duplicateName);
      setStatus({ kind: "success", message: `Duplicated snippet: ${duplicated?.Name || duplicateName}` });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [
    activeProjectResult,
    refreshProjectSnippets,
    snippet,
    snippetNameInput,
    snippets,
  ]);

  const handleRenameSnippet = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before renaming snippets." });
      return;
    }
    if (!selectedSnippetId) {
      setStatus({ kind: "info", message: "Select a snippet to rename." });
      return;
    }
    if (!snippetNameInput.trim()) {
      setStatus({ kind: "error", message: "Snippet name is required." });
      return;
    }

    setIsBusy(true);
    try {
      const renamed = normalizeSnippetRecord(await saveProjectSnippet(
        activeProjectResult.Project.Path,
        selectedSnippetId,
        snippetNameInput.trim(),
        snippet,
      ));
      await refreshProjectSnippets(activeProjectResult.Project.Path);
      setSnippetNameInput(renamed?.Name || snippetNameInput.trim());
      setStatus({ kind: "success", message: `Renamed snippet: ${renamed?.Name || snippetNameInput.trim()}` });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [
    activeProjectResult,
    refreshProjectSnippets,
    selectedSnippetId,
    snippet,
    snippetNameInput,
  ]);

  const handleDeleteSnippet = useCallback(async () => {
    if (!activeProjectResult?.Project?.Path) {
      setStatus({ kind: "error", message: "Open a project before deleting snippets." });
      return;
    }
    if (!selectedSnippetId) {
      setStatus({ kind: "info", message: "Select a snippet to delete." });
      return;
    }

    setIsBusy(true);
    try {
      await deleteProjectSnippet(activeProjectResult.Project.Path, selectedSnippetId);
      await refreshProjectSnippets(activeProjectResult.Project.Path);
      setSelectedSnippetId("");
      setSnippetNameInput("");
      setStatus({ kind: "success", message: "Snippet deleted." });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [activeProjectResult, refreshProjectSnippets, selectedSnippetId]);

  const handleFormatSnippet = useCallback(async () => {
    // Prefer LSP-based formatting (gopls: goimports + gofmt) via Monaco action
    const editor = editorAppRef.current?.getEditor?.();
    if (editor) {
      try {
        const action = editor.getAction("editor.action.formatDocument");
        if (action) {
          await action.run();
          setStatus({ kind: "success", message: "Formatted via gopls." });
          return;
        }
      } catch (lspErr) {
        console.warn("LSP format failed, falling back to gofmt", lspErr);
      }
    }
    // Fallback to backend format.Source()
    setIsBusy(true);
    try {
      const formatted = await formatSnippet(snippet);
      setEditorContent(formatted);
      setStatus({ kind: "success", message: "Snippet formatted with gofmt." });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [snippet, setEditorContent]);

  const handlePlaygroundShare = useCallback(async () => {
    const source = snippetRef.current;
    if (!source?.trim()) {
      setStatus({ kind: "error", message: "Nothing to share." });
      return;
    }
    setIsBusy(true);
    setStatus({ kind: "info", message: "Sharing to Go Playground..." });
    try {
      const result = await playgroundShare(source);
      const url = result?.url || result?.URL || result?.Url || "";
      if (url) {
        await navigator.clipboard.writeText(url);
        setStatus({ kind: "success", message: `Shared! URL copied: ${url}` });
      } else {
        setStatus({ kind: "success", message: "Shared to Go Playground." });
      }
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, []);

  const handlePlaygroundImportOpen = useCallback(() => {
    setImportDialogValue("");
    setImportDialogOpen(true);
  }, []);

  const handlePlaygroundImportSubmit = useCallback(async (url) => {
    setImportDialogOpen(false);
    if (!url?.trim()) return;
    setIsBusy(true);
    setStatus({ kind: "info", message: "Importing from Go Playground..." });
    try {
      const source = await playgroundImport(url);
      setEditorContent(source);
      setStatus({ kind: "success", message: "Imported from Go Playground." });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    } finally {
      setIsBusy(false);
    }
  }, [setEditorContent]);

  const executeRun = useCallback(
    async (sourceToRun) => {
      setIsBusy(true);
      setStatus({ kind: "info", message: "Running snippet..." });
      setRunState("running");
      try {
        const runId = `run_${Date.now()}_${Math.random().toString(16).slice(2)}`;
        activeRunIdRef.current = runId;
        setActiveRunId(runId);
        setRunResult(emptyRunResult());
        setLastRunSource(sourceToRun);
        const projectPath = activeProjectResult?.Project?.Path || "";
        const packagePath = projectPath
          ? selectedTarget || activeProjectResult.Project.DefaultPkg || ""
          : "";
        const result = await runSnippet({
          runId,
          projectPath,
          packagePath,
          source: sourceToRun,
        });
        setRunResult(result);
        const blocks = Array.isArray(result.RichBlocks) ? result.RichBlocks : [];
        setOutputTab(blocks.length > 0 ? "rich" : "raw");
        if (result.Canceled) {
          setRunState("canceled");
          setStatus({ kind: "info", message: "Run canceled." });
        } else if (result.TimedOut) {
          setRunState("failed");
          setStatus({
            kind: "error",
            message: `Run timed out after ${formatDurationMs(result.DurationMS)}.`,
          });
        } else if (result.ExitCode === 0) {
          setRunState("success");
          setStatus({
            kind: "success",
            message: `Run completed in ${formatDurationMs(result.DurationMS)}.`,
          });
        } else {
          setRunState("failed");
          setStatus({
            kind: "error",
            message: `Run failed (exit ${formatExitCode(result.ExitCode)}) in ${formatDurationMs(result.DurationMS)}.`,
          });
        }
      } catch (error) {
        setRunState("failed");
        setStatus({ kind: "error", message: normalizeError(error) });
      } finally {
        activeRunIdRef.current = "";
        setActiveRunId("");
        setIsBusy(false);
      }
    },
    [activeProjectResult, selectedTarget],
  );

  const snippetRef = useRef(snippet);
  snippetRef.current = snippet;

  const handleRunSnippet = useCallback(async () => {
    await executeRun(snippetRef.current);
  }, [executeRun]);

  // Keep refs updated for keyboard handlers
  runHandlerRef.current = handleRunSnippet;

  const activeFilePathRef = useRef(activeFilePath);
  activeFilePathRef.current = activeFilePath;

  const handleSaveFileToDisk = useCallback(async () => {
    const filePath = activeFilePathRef.current;
    if (!filePath) return;
    try {
      await saveGoFile(filePath, snippetRef.current);
      setStatus({ kind: "success", message: `Saved: ${filePath}` });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    }
  }, []);
  saveFileHandlerRef.current = handleSaveFileToDisk;

  const handleRerunLast = useCallback(async () => {
    if (!lastRunSource) {
      setStatus({ kind: "info", message: "No previous snippet run yet." });
      return;
    }
    await executeRun(lastRunSource);
  }, [executeRun, lastRunSource]);

  const handleCancelRun = useCallback(async () => {
    const runId = activeRunIdRef.current;
    if (!runId) {
      setStatus({ kind: "info", message: "No active run to cancel." });
      return;
    }
    try {
      await cancelRun(runId);
      setStatus({ kind: "info", message: "Cancel requested..." });
    } catch (error) {
      setStatus({ kind: "error", message: normalizeError(error) });
    }
  }, []);

  const handleJumpToDiagnostic = useCallback((diagnostic) => {
    if (!diagnostic || diagnostic.line <= 0) {
      setStatus({ kind: "error", message: "Diagnostic line mapping is invalid." });
      return;
    }
    setStatus({
      kind: "info",
      message: `Diagnostic at line ${diagnostic.line}${diagnostic.column > 0 ? `:${diagnostic.column}` : ""}: ${diagnostic.message}`,
    });
  }, []);

  // ── Native toolbar JS bridge ─────────────────────────────────────────────
  const toolbarHandlers = useRef({});
  toolbarHandlers.current = {
    toggleSidebar: () => setSidebarOpen((v) => !v),
    openFolder: () => void handlePickDirectory(),
    openFile: () => void handlePickGoFile(),
    newSnippet: handleCreateNewSnippet,
    format: () => void handleFormatSnippet(),
    run: () =>
      runState === "running"
        ? void handleCancelRun()
        : void handleRunSnippet(),
    rerun: () => void handleRerunLast(),
    share: () => void handlePlaygroundShare(),
    import: () => void handlePlaygroundImportOpen(),
    settings: () => setSettingsOpen((v) => !v),
  };

  const handleToolbarAction = useCallback((action) => {
    const handler = toolbarHandlers.current[action];
    if (handler) handler();
  }, []);

  useEffect(() => {
    window.__gopokeToolbarAction = (action) => {
      const handler = toolbarHandlers.current[action];
      if (handler) handler();
    };
    return () => {
      delete window.__gopokeToolbarAction;
    };
  }, []);

  const displayRunResult = runResult || emptyRunResult();
  const activeProjectPath = activeProjectResult?.Project?.Path || "";

  // Sidebar tab helpers
  const sidebarTabs = [
    { id: "project", label: "Project" },
    { id: "snippets", label: "Snippets" },
    { id: "env", label: "Env" },
    { id: "recent", label: "Recent" },
    { id: "help", label: "Help" },
  ];

  return (
    <main className="app-shell" data-platform={platform}>
      {platform === "darwin" && <div className="macos-drag-bar" />}
      {platform !== "darwin" && (
        <Toolbar runState={runState} onAction={handleToolbarAction} />
      )}
      <div className="main-content">
        {sidebarOpen && (
          <aside className="sidebar">
            <div className="sidebar-tabs">
              {sidebarTabs.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  className={`sidebar-tab ${sidebarTab === tab.id ? "active" : ""}`}
                  onClick={() => setSidebarTab(tab.id)}
                >
                  {tab.label}
                </button>
              ))}
            </div>
            <div className="sidebar-body">
              {/* Project tab */}
              {sidebarTab === "project" && activeProjectResult && (
                <>
                  <div className="sidebar-section">
                    <h2>Project Info</h2>
                    <div className="sidebar-meta">
                      <div className="label">Path</div>
                      <div>{activeProjectResult.Project.Path}</div>
                      <div className="label">Module</div>
                      <div>{activeProjectResult.Module.HasModule ? "go.mod detected" : "No go.mod"}</div>
                      <div className="label">Default Pkg</div>
                      <div>{activeProjectResult.Project.DefaultPkg || "(none)"}</div>
                      <div className="label">Working Dir</div>
                      <div>{activeProjectResult.Project.WorkingDir || "(auto)"}</div>
                      <div className="label">Toolchain</div>
                      <div>{activeProjectResult.Project.Toolchain || "go"}</div>
                    </div>
                  </div>

                  <div className="sidebar-section">
                    <h2>Run Target</h2>
                    <div className="sidebar-field">
                      <label htmlFor="target-select">Default target</label>
                      <select
                        id="target-select"
                        value={selectedTarget}
                        onChange={(event) => setSelectedTarget(event.target.value)}
                        disabled={isBusy || !activeProjectResult.Targets || activeProjectResult.Targets.length === 0}
                      >
                        {activeProjectResult.Targets && activeProjectResult.Targets.length > 0 ? (
                          activeProjectResult.Targets.map((target) => (
                            <option key={target.Package} value={target.Package}>
                              {target.Package} ({target.Command})
                            </option>
                          ))
                        ) : (
                          <option value="">No runnable targets</option>
                        )}
                      </select>
                      <button
                        className="secondary"
                        type="button"
                        onClick={() => void handleSaveDefaultTarget()}
                        disabled={isBusy || !activeProjectResult.Targets || activeProjectResult.Targets.length === 0}
                      >
                        Save Default
                      </button>
                    </div>
                  </div>

                  <div className="sidebar-section">
                    <h2>Working Directory</h2>
                    <div className="sidebar-field">
                      <select
                        id="working-directory-select"
                        value={workingDirectory}
                        onChange={(event) => setWorkingDirectory(event.target.value)}
                        disabled={isBusy || workingDirectoryOptions.length === 0}
                      >
                        {workingDirectoryOptions.length > 0 ? (
                          workingDirectoryOptions.map((path) => (
                            <option key={path} value={path}>{path}</option>
                          ))
                        ) : (
                          <option value="">No directories available</option>
                        )}
                      </select>
                      <button
                        className="secondary"
                        type="button"
                        onClick={() => void handleSaveWorkingDirectory()}
                        disabled={isBusy || !workingDirectory}
                      >
                        Save
                      </button>
                    </div>
                  </div>

                  <div className="sidebar-section">
                    <h2>Toolchain</h2>
                    <div className="sidebar-field">
                      <select
                        id="toolchain-select"
                        value={selectedToolchain}
                        onChange={(event) => setSelectedToolchain(event.target.value)}
                        disabled={isBusy || toolchains.length === 0}
                      >
                        {toolchains.length > 0 ? (
                          toolchains.map((toolchain) => (
                            <option key={toolchain.Path} value={toolchain.Path}>
                              {toolchain.Name} ({toolchain.Version})
                            </option>
                          ))
                        ) : (
                          <option value={selectedToolchain}>{selectedToolchain || "go"}</option>
                        )}
                      </select>
                      <button
                        className="secondary"
                        type="button"
                        onClick={() => void handleSaveToolchain()}
                        disabled={isBusy || !selectedToolchain}
                      >
                        Save
                      </button>
                    </div>
                  </div>
                </>
              )}

              {sidebarTab === "project" && !activeProjectResult && (
                <div className="sidebar-section">
                  <p style={{ color: "var(--text-muted)", fontSize: 12 }}>Open a project to see settings.</p>
                </div>
              )}

              {/* Snippets tab */}
              {sidebarTab === "snippets" && activeProjectResult && (
                <>
                  <div className="sidebar-section">
                    <div className="sidebar-field">
                      <label htmlFor="snippet-search">Search</label>
                      <input
                        id="snippet-search"
                        type="text"
                        placeholder="Search by name or content"
                        value={snippetSearch}
                        onChange={(event) => setSnippetSearch(event.target.value)}
                      />
                    </div>
                  </div>

                  <div className="sidebar-section">
                    <div className="sidebar-field">
                      <label htmlFor="snippet-name">Snippet Name</label>
                      <input
                        id="snippet-name"
                        type="text"
                        placeholder="Snippet name"
                        value={snippetNameInput}
                        onChange={(event) => setSnippetNameInput(event.target.value)}
                      />
                    </div>
                    <div className="sidebar-field-row">
                      <button className="secondary" type="button" onClick={handleCreateNewSnippet} disabled={isBusy}>New</button>
                      <button type="button" onClick={() => void handleSaveSnippet()} disabled={isBusy || !snippetNameInput.trim()}>Save</button>
                      <button className="secondary" type="button" onClick={() => void handleDuplicateSnippet()} disabled={isBusy || (!selectedSnippetId && !snippetNameInput.trim())}>Dup</button>
                      <button className="secondary" type="button" onClick={() => void handleDeleteSnippet()} disabled={isBusy || !selectedSnippetId}>Del</button>
                    </div>
                  </div>

                  <ul className="sidebar-list">
                    {filteredSnippets.length > 0 ? (
                      filteredSnippets.map((item) => (
                        <li key={item.ID}>
                          <button
                            type="button"
                            className={`sidebar-list-item secondary ${selectedSnippetId === item.ID ? "selected" : ""}`}
                            onClick={() => handleSelectSnippet(item)}
                            disabled={isBusy}
                          >
                            <span className="item-name">{item.Name}</span>
                            <span className="item-meta">{formatDateTime(item.UpdatedAt)}</span>
                          </button>
                        </li>
                      ))
                    ) : (
                      <li style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0" }}>No snippets found.</li>
                    )}
                  </ul>
                </>
              )}

              {sidebarTab === "snippets" && !activeProjectResult && (
                <div className="sidebar-section">
                  <p style={{ color: "var(--text-muted)", fontSize: 12 }}>Open a project to manage snippets.</p>
                </div>
              )}

              {/* Env tab */}
              {sidebarTab === "env" && activeProjectResult && (
                <>
                  <div className="sidebar-section">
                    <h2>Add / Edit Variable</h2>
                    <div className="sidebar-field">
                      <label htmlFor="env-key">Key</label>
                      <input
                        id="env-key"
                        type="text"
                        placeholder="ENV_KEY"
                        value={envKeyInput}
                        onChange={(event) => setEnvKeyInput(event.target.value)}
                      />
                    </div>
                    <div className="sidebar-field">
                      <label htmlFor="env-value">Value</label>
                      <input
                        id="env-value"
                        type="text"
                        placeholder="value"
                        value={envValueInput}
                        onChange={(event) => setEnvValueInput(event.target.value)}
                      />
                    </div>
                    <div className="sidebar-field-row">
                      <label htmlFor="env-masked" className="inline-label">
                        <input
                          id="env-masked"
                          type="checkbox"
                          checked={envMaskedInput}
                          onChange={(event) => setEnvMaskedInput(event.target.checked)}
                        />
                        Mask
                      </label>
                      <button
                        type="button"
                        onClick={() => void handleSaveEnvVar()}
                        disabled={isBusy || !envKeyInput.trim()}
                      >
                        Save
                      </button>
                    </div>
                  </div>

                  <ul className="sidebar-list">
                    {envVars.length > 0 ? (
                      envVars.map((item) => {
                        const revealed = envRevealMap[item.Key] === true;
                        const displayValue = item.Masked && !revealed ? "********" : item.Value;
                        return (
                          <li key={item.Key}>
                            <div className="list-row">
                              <span className="item-name">{item.Key}</span>
                              <span className="item-meta">{displayValue || "(empty)"}</span>
                              <div className="list-actions">
                                {item.Masked && (
                                  <button
                                    type="button"
                                    className="secondary"
                                    onClick={() =>
                                      setEnvRevealMap((current) => ({
                                        ...current,
                                        [item.Key]: !(current[item.Key] === true),
                                      }))
                                    }
                                  >
                                    {revealed ? "Hide" : "Show"}
                                  </button>
                                )}
                                <button
                                  type="button"
                                  className="secondary"
                                  onClick={() => {
                                    setEnvKeyInput(item.Key);
                                    setEnvValueInput(item.Value);
                                    setEnvMaskedInput(item.Masked);
                                  }}
                                >
                                  Edit
                                </button>
                                <button
                                  type="button"
                                  className="secondary"
                                  onClick={() => void handleDeleteEnvVar(item.Key)}
                                  disabled={isBusy}
                                >
                                  Del
                                </button>
                              </div>
                            </div>
                          </li>
                        );
                      })
                    ) : (
                      <li style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0" }}>No env vars defined.</li>
                    )}
                  </ul>
                </>
              )}

              {sidebarTab === "env" && !activeProjectResult && (
                <div className="sidebar-section">
                  <p style={{ color: "var(--text-muted)", fontSize: 12 }}>Open a project to manage env vars.</p>
                </div>
              )}

              {/* Recent tab */}
              {sidebarTab === "recent" && (
                <ul className="sidebar-list">
                  {recent.length > 0 ? (
                    recent.map((project) => (
                      <li key={project.ID || project.Path}>
                        <button
                          className="sidebar-list-item secondary"
                          type="button"
                          onClick={() => void handleOpenProject(project.Path)}
                          disabled={isBusy}
                        >
                          <span className="item-name">{project.Path}</span>
                          <span className="item-meta">Last: {formatDateTime(project.LastOpenedAt)}</span>
                        </button>
                      </li>
                    ))
                  ) : (
                    <li style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0" }}>No recent projects.</li>
                  )}
                </ul>
              )}

              {/* Help tab */}
              {sidebarTab === "help" && (
                <div className="sidebar-section">
                  <h2>Quick Start</h2>
                  <ol className="help-checklist">
                    <li>Open a Go project with Open Folder, or open a single .go file with Open File.</li>
                    <li>Run your snippet with Run (Cmd+Enter).</li>
                    <li>Cancel long runs by clicking the stop button.</li>
                    <li>Save useful code from the Snippets tab.</li>
                  </ol>
                  <details className="help-details">
                    <summary>Keyboard Shortcuts</summary>
                    <ul className="help-tips">
                      <li>Cmd+S — Save file to disk (when a .go file is open)</li>
                      <li>Cmd+B — Toggle sidebar</li>
                      <li>Cmd+1 — Snippets tab</li>
                      <li>Cmd+2 — Env tab</li>
                      <li>Cmd+3 — Project tab</li>
                      <li>Cmd+4 — Recent tab</li>
                      <li>Cmd+Enter — Run snippet</li>
                    </ul>
                  </details>
                  <details className="help-details">
                    <summary>Rich Output</summary>
                    <p style={{ color: "var(--text-soft)", fontSize: 12, margin: "6px 0 4px" }}>
                      Print special markers to render tables and JSON cards in the Rich tab.
                    </p>
                    <ul className="help-tips">
                      <li><strong>Table:</strong> <code>{"//gopoke:table [{\"col\":\"val\"}]"}</code></li>
                      <li><strong>JSON card:</strong> <code>{"//gopoke:json {\"key\":\"val\"}"}</code></li>
                      <li>Marker lines are stripped from Raw output.</li>
                      <li>Rich tab auto-selects when blocks are present.</li>
                      <li>Malformed JSON stays in raw output (no block created).</li>
                      <li>Unknown types show a fallback with raw JSON.</li>
                    </ul>
                  </details>
                  <details className="help-details">
                    <summary>Tips</summary>
                    <ul className="help-tips">
                      <li>Use Format (gofmt) before run for cleaner diagnostics.</li>
                      <li>Set toolchain/working directory in the Project tab.</li>
                      <li>Use masked env vars for secrets.</li>
                    </ul>
                  </details>
                </div>
              )}
            </div>
          </aside>
        )}

        <div className="editor-pane">
          <GopokeMonacoEditor
            code={snippet}
            onCodeChange={setSnippet}
            wsPort={lspPort}
            workspaceDir={lspWorkspaceDir}
            theme={editorSettings.theme}
            fontSize={editorSettings.fontSize}
            fontFamily={editorSettings.fontFamily}
            lineNumbers={editorSettings.lineNumbers ? "on" : "off"}
            onEditorReady={(app) => { editorAppRef.current = app; }}
          />
          <div className="editor-status">
            <span>{lineCount} lines</span>
            <span>{snippet.length} chars</span>
            {selectedSnippetId && <span>{snippetNameInput}</span>}
            {activeFilePath && <span title={activeFilePath}>{activeFilePath.split("/").pop()}</span>}
            <span className="status-spacer" />
            <span className={`status-run-state ${runState}`}>{runStateLabel(runState)}</span>
            {runResult && runResult.DurationMS != null && (
              <span className="status-duration">{formatDurationMs(runResult.DurationMS)}</span>
            )}
            {runResult && runResult.ExitCode != null && (
              <span className={`status-exit-code ${runResult.ExitCode === 0 ? "exit-ok" : "exit-err"}`}>
                exit {formatExitCode(runResult.ExitCode)}
              </span>
            )}
            <span className={`status-message ${status.kind}`} title={status.message}>
              {status.message}
            </span>
          </div>
        </div>

        <div className="pane-separator" />

        <div className="output-pane">
          {diagnostics.length > 0 && (
            <div className="diagnostics-bar">
              <button
                type="button"
                className={`diagnostics-bar-toggle ${diagExpanded ? "expanded" : ""}`}
                onClick={() => setDiagExpanded((v) => !v)}
              >
                <span className="diag-count">{diagnostics.length}</span>
                <span>Diagnostics</span>
                <svg className="diag-chevron" width="10" height="10" viewBox="0 0 10 10" fill="none"><path d="M2.5 3.75 5 6.25 7.5 3.75" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"/></svg>
              </button>
              {diagExpanded && (
                <div className="diagnostics-list">
                  <ul>
                    {diagnostics.map((diagnostic, index) => (
                      <li key={`${diagnostic.kind}:${diagnostic.file}:${diagnostic.line}:${diagnostic.column}:${index}`}>
                        <button
                          type="button"
                          className="diagnostic-item secondary"
                          onClick={() => handleJumpToDiagnostic(diagnostic)}
                        >
                          {diagnosticTitle(diagnostic)}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
          {runResult ? (
            <>
              {hasRichBlocks && (
                <div className="output-tabs">
                  <button
                    type="button"
                    className={`output-tab ${outputTab === "rich" ? "active" : ""}`}
                    onClick={() => setOutputTab("rich")}
                  >
                    Rich
                  </button>
                  <button
                    type="button"
                    className={`output-tab ${outputTab === "raw" ? "active" : ""}`}
                    onClick={() => setOutputTab("raw")}
                  >
                    Raw
                  </button>
                </div>
              )}
              {outputTab === "rich" && hasRichBlocks ? (
                <div className="output-rich-scroll">
                  <RichOutput blocks={richBlocks} />
                </div>
              ) : (
                <pre className="output-content">{combinedOutput || "(no output)"}</pre>
              )}
            </>
          ) : (
            <div className="output-empty">Run a snippet to see output</div>
          )}
        </div>
      </div>

      {importDialogOpen && (
        <>
          <div className="import-dialog-overlay" onClick={() => setImportDialogOpen(false)} />
          <div
            className="import-dialog"
            onKeyDown={(e) => {
              if (e.key === "Escape") setImportDialogOpen(false);
              if (e.key === "Enter") handlePlaygroundImportSubmit(importDialogValue);
            }}
          >
            <h3>Import from Go Playground</h3>
            <input
              type="text"
              className="import-dialog-input"
              placeholder="URL or hash, e.g. https://go.dev/play/p/abc123"
              value={importDialogValue}
              onChange={(e) => setImportDialogValue(e.target.value)}
              autoFocus
            />
            <div className="import-dialog-buttons">
              <button
                type="button"
                className="import-dialog-btn cancel"
                onClick={() => setImportDialogOpen(false)}
              >
                Cancel
              </button>
              <button
                type="button"
                className="import-dialog-btn confirm"
                onClick={() => handlePlaygroundImportSubmit(importDialogValue)}
                disabled={!importDialogValue.trim()}
              >
                Import
              </button>
            </div>
          </div>
        </>
      )}

      {settingsOpen && (
        <SettingsPanel
          onClose={() => setSettingsOpen(false)}
          editorSettings={editorSettings}
          onEditorSettingChange={updateEditorSetting}
        />
      )}
    </main>
  );
}

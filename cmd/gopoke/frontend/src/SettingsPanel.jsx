import { useCallback, useEffect, useState } from "react";
import {
  getGlobalSettings,
  updateGlobalSettings,
  detectToolVersions,
  listGoVersions,
  downloadGoSDK,
  downloadGopls,
  downloadStaticcheck,
  browseForBinary,
  onToolchainProgress,
  onToolchainComplete,
  onToolchainError,
} from "./wailsBridge";

// Theme metadata: id, group, and 5 swatch colors (bg, fg, keyword, string, accent)
const monacoThemes = [
  { id: "Default Dark Modern", group: "dark", colors: ["#1f1f1f", "#cccccc", "#569cd6", "#ce9178", "#007acc"] },
  { id: "GitHub Dark Default", group: "dark", colors: ["#0d1117", "#e6edf3", "#ff7b72", "#8b949e", "#2f81f7"] },
  { id: "One Dark Pro", group: "dark", colors: ["#282c34", "#abb2bf", "#c678dd", "#98c379", "#528bff"] },
  { id: "Dracula", group: "dark", colors: ["#282A36", "#F8F8F2", "#BD93F9", "#FF79C6", "#6272A4"] },
  { id: "Monokai", group: "dark", colors: ["#272822", "#f8f8f2", "#F92672", "#E6DB74", "#A6E22E"] },
  { id: "Material Theme Darker", group: "dark", colors: ["#212121", "#EEFFFF", "#C792EA", "#C3E88D", "#FFCC00"] },
  { id: "Nord", group: "dark", colors: ["#2e3440", "#d8dee9", "#81A1C1", "#A3BE8C", "#88C0D0"] },
  { id: "Catppuccin Mocha", group: "dark", colors: ["#1e1e2e", "#cdd6f4", "#fab387", "#a6e3a1", "#f5e0dc"] },
  { id: "Solarized Dark", group: "dark", colors: ["#002B36", "#839496", "#859900", "#2AA198", "#D30102"] },
  { id: "Default Light Modern", group: "light", colors: ["#ffffff", "#3b3b3b", "#0000ff", "#a31515", "#007acc"] },
  { id: "GitHub Light Default", group: "light", colors: ["#ffffff", "#1f2328", "#cf222e", "#6e7781", "#0969da"] },
  { id: "Solarized Light", group: "light", colors: ["#FDF6E3", "#657B83", "#859900", "#2AA198", "#657B83"] },
  { id: "Default High Contrast", group: "hc", colors: ["#000000", "#ffffff", "#569cd6", "#ce9178", "#ffffff"] },
  { id: "Default High Contrast Light", group: "hc", colors: ["#ffffff", "#000000", "#0000ff", "#a31515", "#000000"] },
];

const themeGroups = [
  { label: "Dark", themes: monacoThemes.filter((t) => t.group === "dark") },
  { label: "Light", themes: monacoThemes.filter((t) => t.group === "light") },
  { label: "High Contrast", themes: monacoThemes.filter((t) => t.group === "hc") },
];

const themeColorsByName = Object.fromEntries(monacoThemes.map((t) => [t.id, t.colors]));

const tabs = [
  { id: "toolchains", label: "Toolchains" },
  { id: "editor", label: "Editor" },
  { id: "advanced", label: "Advanced" },
];

/**
 * SettingsPanel — tabbed settings panel replacing the old editor-only modal.
 *
 * Props:
 *   onClose        — callback to close the panel
 *   editorSettings — current editor settings (for immediate Monaco updates)
 *   onEditorSettingChange — callback(key, value) to update editor setting in parent
 */
export default function SettingsPanel({ onClose, editorSettings, onEditorSettingChange }) {
  const [activeTab, setActiveTab] = useState("toolchains");

  // Global settings from backend
  const [settings, setSettings] = useState(null);
  const [saving, setSaving] = useState(false);

  // Toolchain detection
  const [toolVersions, setToolVersions] = useState(null);
  const [detecting, setDetecting] = useState(false);

  // Go version list for SDK download
  const [goVersions, setGoVersions] = useState([]);
  const [selectedGoVersion, setSelectedGoVersion] = useState("");
  const [loadingVersions, setLoadingVersions] = useState(false);

  // Download progress per tool
  const [downloads, setDownloads] = useState({});

  // Advanced tab local edits (buffered before save)
  const [advancedDraft, setAdvancedDraft] = useState(null);

  // Load settings and tool versions on mount
  useEffect(() => {
    loadSettings();
    detectTools();
  }, []);

  // Subscribe to toolchain download events
  useEffect(() => {
    const unsubs = [
      onToolchainProgress((data) => {
        const d = normalizeEvent(data);
        if (!d.tool) return;
        setDownloads((prev) => ({
          ...prev,
          [d.tool]: { stage: d.stage, percent: d.percent || 0, message: d.message || "" },
        }));
      }),
      onToolchainComplete((data) => {
        const d = normalizeEvent(data);
        if (!d.tool) return;
        setDownloads((prev) => {
          const next = { ...prev };
          delete next[d.tool];
          return next;
        });
        // Re-detect after completion
        detectTools();
        loadSettings();
      }),
      onToolchainError((data) => {
        const d = normalizeEvent(data);
        if (!d.tool) return;
        setDownloads((prev) => ({
          ...prev,
          [d.tool]: { stage: "error", percent: 0, message: d.message || "Download failed" },
        }));
      }),
    ];
    return () => unsubs.forEach((fn) => fn());
  }, []);

  async function loadSettings() {
    try {
      const s = await getGlobalSettings();
      setSettings(s);
      setAdvancedDraft({
        defaultTimeoutMS: s.defaultTimeoutMS || 30000,
        maxOutputBytes: s.maxOutputBytes || 1048576,
        goPathOverride: s.goPathOverride || "",
        goModCacheOverride: s.goModCacheOverride || "",
      });

      // Migrate localStorage editor settings to backend on first load
      migrateLocalStorage(s);
    } catch (err) {
      console.error("Failed to load settings:", err);
    }
  }

  function migrateLocalStorage(backendSettings) {
    try {
      const raw = localStorage.getItem("gopoke:editor-settings");
      if (!raw) return;
      const local = JSON.parse(raw);
      // If backend has defaults and localStorage has user values, migrate
      const needsMigration =
        backendSettings.editorTheme === "" ||
        backendSettings.editorTheme === "Default Dark Modern";
      if (needsMigration && local.theme && local.theme !== "Default Dark Modern") {
        updateSetting("editorTheme", local.theme);
      }
      if (needsMigration && local.fontFamily && local.fontFamily !== "JetBrains Mono") {
        updateSetting("editorFontFamily", local.fontFamily);
      }
      if (needsMigration && local.fontSize && local.fontSize !== 14) {
        updateSetting("editorFontSize", local.fontSize);
      }
      if (needsMigration && local.lineNumbers === false) {
        updateSetting("editorLineNumbers", false);
      }
      // Clear localStorage after migration
      localStorage.removeItem("gopoke:editor-settings");
    } catch {}
  }

  async function detectTools() {
    setDetecting(true);
    try {
      const v = await detectToolVersions();
      setToolVersions(v);
    } catch (err) {
      console.error("Failed to detect tools:", err);
    } finally {
      setDetecting(false);
    }
  }

  async function updateSetting(key, value) {
    if (!settings) return;
    const updated = { ...settings, [key]: value };
    setSettings(updated);
    try {
      await updateGlobalSettings(updated);
    } catch (err) {
      console.error("Failed to save setting:", err);
    }
  }

  // Editor setting change: update both backend and parent (for live Monaco preview)
  const handleEditorSettingChange = useCallback(
    (key, value) => {
      // Map from backend field names to parent's editorSettings keys
      const backendToLocal = {
        editorTheme: "theme",
        editorFontFamily: "fontFamily",
        editorFontSize: "fontSize",
        editorLineNumbers: "lineNumbers",
      };
      updateSetting(key, value);
      if (backendToLocal[key] && onEditorSettingChange) {
        onEditorSettingChange(backendToLocal[key], value);
      }
    },
    [settings, onEditorSettingChange],
  );

  async function handleBrowse(tool, settingKey) {
    try {
      const path = await browseForBinary(`Select ${tool} binary`);
      if (path) {
        await updateSetting(settingKey, path);
        detectTools();
      }
    } catch (err) {
      console.error("Browse failed:", err);
    }
  }

  async function handleDownloadGoSDK() {
    if (!selectedGoVersion) {
      // Load versions if not loaded yet
      if (goVersions.length === 0) {
        await handleLoadGoVersions();
      }
      return;
    }
    try {
      await downloadGoSDK(selectedGoVersion);
    } catch (err) {
      console.error("Go SDK download failed:", err);
    }
  }

  async function handleLoadGoVersions() {
    setLoadingVersions(true);
    try {
      const versions = await listGoVersions();
      setGoVersions(versions || []);
      if (versions && versions.length > 0) {
        setSelectedGoVersion(versions[0].version || versions[0].Version || "");
      }
    } catch (err) {
      console.error("Failed to load Go versions:", err);
    } finally {
      setLoadingVersions(false);
    }
  }

  async function handleDownloadGopls() {
    try {
      await downloadGopls();
    } catch (err) {
      console.error("gopls install failed:", err);
    }
  }

  async function handleDownloadStaticcheck() {
    try {
      await downloadStaticcheck();
    } catch (err) {
      console.error("staticcheck install failed:", err);
    }
  }

  async function handleAdvancedSave() {
    if (!advancedDraft || !settings) return;
    setSaving(true);
    try {
      const updated = {
        ...settings,
        defaultTimeoutMS: Number(advancedDraft.defaultTimeoutMS) || 30000,
        maxOutputBytes: Number(advancedDraft.maxOutputBytes) || 1048576,
        goPathOverride: advancedDraft.goPathOverride,
        goModCacheOverride: advancedDraft.goModCacheOverride,
      };
      await updateGlobalSettings(updated);
      setSettings(updated);
    } catch (err) {
      console.error("Failed to save advanced settings:", err);
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <div className="settings-overlay" onClick={onClose} />
      <div
        className="settings-panel"
        onKeyDown={(e) => { if (e.key === "Escape") onClose(); }}
      >
        <div className="settings-header">
          <h2>Settings</h2>
          <button type="button" className="settings-close" onClick={onClose}>
            &times;
          </button>
        </div>

        <div className="settings-tabs">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              className={`settings-tab${activeTab === tab.id ? " active" : ""}`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="settings-body">
          {activeTab === "toolchains" && (
            <ToolchainsTab
              settings={settings}
              toolVersions={toolVersions}
              detecting={detecting}
              downloads={downloads}
              goVersions={goVersions}
              selectedGoVersion={selectedGoVersion}
              loadingVersions={loadingVersions}
              onSelectGoVersion={setSelectedGoVersion}
              onLoadGoVersions={handleLoadGoVersions}
              onDownloadGoSDK={handleDownloadGoSDK}
              onDownloadGopls={handleDownloadGopls}
              onDownloadStaticcheck={handleDownloadStaticcheck}
              onBrowse={handleBrowse}
              onDetect={detectTools}
              onUpdateSetting={updateSetting}
            />
          )}

          {activeTab === "editor" && (
            <EditorTab
              editorSettings={editorSettings}
              onSettingChange={handleEditorSettingChange}
            />
          )}

          {activeTab === "advanced" && (
            <AdvancedTab
              draft={advancedDraft}
              onDraftChange={setAdvancedDraft}
              onSave={handleAdvancedSave}
              saving={saving}
            />
          )}
        </div>
      </div>
    </>
  );
}

// ── Toolchains Tab ──────────────────────────────────────

function ToolchainsTab({
  settings,
  toolVersions,
  detecting,
  downloads,
  goVersions,
  selectedGoVersion,
  loadingVersions,
  onSelectGoVersion,
  onLoadGoVersions,
  onDownloadGoSDK,
  onDownloadGopls,
  onDownloadStaticcheck,
  onBrowse,
  onDetect,
  onUpdateSetting,
}) {
  return (
    <div className="settings-toolchains">
      <div className="settings-section">
        <div className="toolchain-header">
          <h3>Toolchains</h3>
          <button
            type="button"
            className="toolchain-detect-btn"
            onClick={onDetect}
            disabled={detecting}
          >
            {detecting ? "Detecting..." : "Re-detect"}
          </button>
        </div>
      </div>

      <ToolchainRow
        label="Go SDK"
        path={settings?.goPath || ""}
        version={toolVersions?.goVersion || ""}
        versionPath={toolVersions?.goPath || ""}
        downloading={downloads.go}
        onPathChange={(v) => onUpdateSetting("goPath", v)}
        onBrowse={() => onBrowse("Go", "goPath")}
        downloadAction={
          <GoSDKDownload
            goVersions={goVersions}
            selectedGoVersion={selectedGoVersion}
            loadingVersions={loadingVersions}
            onSelectGoVersion={onSelectGoVersion}
            onLoadGoVersions={onLoadGoVersions}
            onDownload={onDownloadGoSDK}
          />
        }
      />

      <ToolchainRow
        label="gopls"
        path={settings?.goplsPath || ""}
        version={toolVersions?.goplsVersion || ""}
        versionPath={toolVersions?.goplsPath || ""}
        downloading={downloads.gopls}
        onPathChange={(v) => onUpdateSetting("goplsPath", v)}
        onBrowse={() => onBrowse("gopls", "goplsPath")}
        downloadAction={
          <button
            type="button"
            className="toolchain-download-btn"
            onClick={onDownloadGopls}
            disabled={!!downloads.gopls}
          >
            Install
          </button>
        }
      />

      <ToolchainRow
        label="staticcheck"
        path={settings?.staticcheckPath || ""}
        version={toolVersions?.staticcheckVersion || ""}
        versionPath={toolVersions?.staticcheckPath || ""}
        downloading={downloads.staticcheck}
        onPathChange={(v) => onUpdateSetting("staticcheckPath", v)}
        onBrowse={() => onBrowse("staticcheck", "staticcheckPath")}
        downloadAction={
          <button
            type="button"
            className="toolchain-download-btn"
            onClick={onDownloadStaticcheck}
            disabled={!!downloads.staticcheck}
          >
            Install
          </button>
        }
      />
    </div>
  );
}

// ── Toolchain Row ───────────────────────────────────────

function ToolchainRow({
  label,
  path,
  version,
  versionPath,
  downloading,
  onPathChange,
  onBrowse,
  downloadAction,
}) {
  const found = !!version;
  const statusClass = downloading
    ? "status-downloading"
    : found
      ? "status-found"
      : "status-missing";
  const statusText = downloading
    ? downloading.stage === "error"
      ? "Error"
      : "Downloading..."
    : found
      ? "Found"
      : "Not Found";

  return (
    <div className="settings-section toolchain-row">
      <div className="toolchain-row-header">
        <h3>{label}</h3>
        <span className={`toolchain-status ${statusClass}`}>{statusText}</span>
      </div>

      {version && (
        <div className="toolchain-version">
          {version}
          {versionPath && <span className="toolchain-version-path">{versionPath}</span>}
        </div>
      )}

      <div className="toolchain-path-row">
        <input
          type="text"
          className="toolchain-path-input"
          placeholder="Auto-detect from PATH"
          value={path}
          onChange={(e) => onPathChange(e.target.value)}
        />
        <button type="button" className="toolchain-browse-btn" onClick={onBrowse}>
          Browse
        </button>
      </div>

      {downloading && downloading.stage !== "error" && (
        <div className="toolchain-progress">
          <div className="toolchain-progress-bar">
            <div
              className="toolchain-progress-fill"
              style={{ width: `${Math.max(2, downloading.percent || 0)}%` }}
            />
          </div>
          <span className="toolchain-progress-text">
            {downloading.message || `${Math.round(downloading.percent || 0)}%`}
          </span>
        </div>
      )}

      {downloading && downloading.stage === "error" && (
        <div className="toolchain-error">{downloading.message}</div>
      )}

      {!downloading && <div className="toolchain-actions">{downloadAction}</div>}
    </div>
  );
}

// ── Go SDK Version Picker ───────────────────────────────

function GoSDKDownload({
  goVersions,
  selectedGoVersion,
  loadingVersions,
  onSelectGoVersion,
  onLoadGoVersions,
  onDownload,
}) {
  if (goVersions.length === 0) {
    return (
      <button
        type="button"
        className="toolchain-download-btn"
        onClick={onLoadGoVersions}
        disabled={loadingVersions}
      >
        {loadingVersions ? "Loading..." : "Download"}
      </button>
    );
  }

  return (
    <div className="go-version-picker">
      <select
        value={selectedGoVersion}
        onChange={(e) => onSelectGoVersion(e.target.value)}
      >
        {goVersions.map((v) => {
          const ver = v.version || v.Version || "";
          return (
            <option key={ver} value={ver}>
              {ver}{v.stable || v.Stable ? "" : " (unstable)"}
            </option>
          );
        })}
      </select>
      <button type="button" className="toolchain-download-btn" onClick={onDownload}>
        Download
      </button>
    </div>
  );
}

// ── Editor Tab ──────────────────────────────────────────

function EditorTab({ editorSettings, onSettingChange }) {
  return (
    <>
      <div className="settings-section">
        <h3>Theme</h3>
        <select
          value={editorSettings.theme}
          onChange={(e) => onSettingChange("editorTheme", e.target.value)}
        >
          {themeGroups.map((group) => (
            <optgroup key={group.label} label={group.label}>
              {group.themes.map((t) => (
                <option key={t.id} value={t.id}>{t.id}</option>
              ))}
            </optgroup>
          ))}
        </select>
        {themeColorsByName[editorSettings.theme] && (
          <>
            <div className="theme-swatch">
              {themeColorsByName[editorSettings.theme].map((color, i) => (
                <div key={i} className="theme-swatch-bar" style={{ background: color }} />
              ))}
            </div>
            <div className="theme-swatch-label">
              <span>bg</span><span>fg</span><span>keyword</span><span>string</span><span>accent</span>
            </div>
          </>
        )}
      </div>

      <div className="settings-section">
        <h3>Font</h3>
        <select
          value={editorSettings.fontFamily}
          onChange={(e) => onSettingChange("editorFontFamily", e.target.value)}
        >
          {["JetBrains Mono", "SF Mono", "Menlo", "Fira Code", "Source Code Pro", "Cascadia Code"].map(
            (font) => <option key={font} value={font}>{font}</option>,
          )}
        </select>
      </div>

      <div className="settings-section">
        <h3>Font Size</h3>
        <div className="settings-stepper">
          <button
            type="button"
            onClick={() => onSettingChange("editorFontSize", Math.max(10, editorSettings.fontSize - 1))}
            disabled={editorSettings.fontSize <= 10}
          >
            &minus;
          </button>
          <span className="stepper-value">{editorSettings.fontSize}px</span>
          <button
            type="button"
            onClick={() => onSettingChange("editorFontSize", Math.min(24, editorSettings.fontSize + 1))}
            disabled={editorSettings.fontSize >= 24}
          >
            +
          </button>
        </div>
      </div>

      <div className="settings-section">
        <h3>Line Numbers</h3>
        <label className="settings-toggle">
          <input
            type="checkbox"
            checked={editorSettings.lineNumbers}
            onChange={(e) => onSettingChange("editorLineNumbers", e.target.checked)}
          />
          <span className="toggle-label">{editorSettings.lineNumbers ? "Visible" : "Hidden"}</span>
        </label>
      </div>
    </>
  );
}

// ── Advanced Tab ────────────────────────────────────────

function AdvancedTab({ draft, onDraftChange, onSave, saving }) {
  if (!draft) return null;

  const updateField = (key, value) => {
    onDraftChange({ ...draft, [key]: value });
  };

  return (
    <>
      <div className="settings-section">
        <h3>Default Timeout (ms)</h3>
        <input
          type="number"
          className="settings-input"
          min={1000}
          max={300000}
          step={1000}
          value={draft.defaultTimeoutMS}
          onChange={(e) => updateField("defaultTimeoutMS", e.target.value)}
        />
      </div>

      <div className="settings-section">
        <h3>Max Output Size (bytes)</h3>
        <input
          type="number"
          className="settings-input"
          min={1024}
          max={10485760}
          step={1024}
          value={draft.maxOutputBytes}
          onChange={(e) => updateField("maxOutputBytes", e.target.value)}
        />
      </div>

      <div className="settings-section">
        <h3>GOPATH Override</h3>
        <input
          type="text"
          className="settings-input"
          placeholder="Leave empty to use system default"
          value={draft.goPathOverride}
          onChange={(e) => updateField("goPathOverride", e.target.value)}
        />
      </div>

      <div className="settings-section">
        <h3>GOMODCACHE Override</h3>
        <input
          type="text"
          className="settings-input"
          placeholder="Leave empty to use system default"
          value={draft.goModCacheOverride}
          onChange={(e) => updateField("goModCacheOverride", e.target.value)}
        />
      </div>

      <div className="settings-section">
        <button
          type="button"
          className="settings-save-btn"
          onClick={onSave}
          disabled={saving}
        >
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </>
  );
}

// ── Helpers ─────────────────────────────────────────────

function normalizeEvent(data) {
  if (!data || typeof data !== "object") return {};
  return {
    tool: data.tool || data.Tool || "",
    stage: data.stage || data.Stage || "",
    percent: data.percent || data.Percent || 0,
    message: data.message || data.Message || "",
  };
}

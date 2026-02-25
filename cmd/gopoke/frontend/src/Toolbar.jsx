// HTML toolbar fallback for non-macOS platforms.
// macOS uses a native NSToolbar; this component renders only on Linux/Windows.

const icons = {
  sidebar: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
      <line x1="5.5" y1="2.5" x2="5.5" y2="13.5" />
    </svg>
  ),
  folder: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1.5 4.5v-1a1 1 0 0 1 1-1h3l1.5 1.5h6a1 1 0 0 1 1 1v7a1 1 0 0 1-1 1h-10.5a1 1 0 0 1-1-1v-7.5z" />
    </svg>
  ),
  file: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9.5 1.5h-5a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1v-8.5z" />
      <polyline points="9.5 1.5 9.5 5 12.5 5" />
    </svg>
  ),
  newFile: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9.5 1.5h-5a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1v-8.5z" />
      <polyline points="9.5 1.5 9.5 5 12.5 5" />
      <line x1="6.5" y1="9.5" x2="9.5" y2="9.5" />
      <line x1="8" y1="8" x2="8" y2="11" />
    </svg>
  ),
  format: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="2.5" y1="3.5" x2="13.5" y2="3.5" />
      <line x1="2.5" y1="6.5" x2="10.5" y2="6.5" />
      <line x1="2.5" y1="9.5" x2="13.5" y2="9.5" />
      <line x1="2.5" y1="12.5" x2="8.5" y2="12.5" />
    </svg>
  ),
  play: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="4.5,2.5 13,8 4.5,13.5" />
    </svg>
  ),
  stop: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="10" height="10" rx="1" />
    </svg>
  ),
  rerun: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12.5 6.5a5 5 0 1 1-1.46-3.04" />
      <polyline points="12.5 2.5 12.5 6.5 8.5 6.5" />
    </svg>
  ),
  share: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 10V2.5" />
      <polyline points="5 5 8 2 11 5" />
      <path d="M3.5 9v4a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1V9" />
    </svg>
  ),
  import: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 2v7.5" />
      <polyline points="5 7 8 10 11 7" />
      <path d="M3.5 11v2a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1v-2" />
    </svg>
  ),
  settings: (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="8" cy="8" r="2" />
      <path d="M13.1 9.9a1.2 1.2 0 0 0 .24 1.32l.04.04a1.45 1.45 0 1 1-2.05 2.05l-.04-.04a1.2 1.2 0 0 0-1.32-.24 1.2 1.2 0 0 0-.73 1.1v.12a1.45 1.45 0 0 1-2.9 0v-.06a1.2 1.2 0 0 0-.78-1.1 1.2 1.2 0 0 0-1.32.24l-.04.04a1.45 1.45 0 1 1-2.05-2.05l.04-.04a1.2 1.2 0 0 0 .24-1.32 1.2 1.2 0 0 0-1.1-.73h-.12a1.45 1.45 0 0 1 0-2.9h.06a1.2 1.2 0 0 0 1.1-.78 1.2 1.2 0 0 0-.24-1.32l-.04-.04A1.45 1.45 0 1 1 4.14 2.1l.04.04a1.2 1.2 0 0 0 1.32.24h.06a1.2 1.2 0 0 0 .73-1.1v-.12a1.45 1.45 0 0 1 2.9 0v.06a1.2 1.2 0 0 0 .73 1.1 1.2 1.2 0 0 0 1.32-.24l.04-.04a1.45 1.45 0 1 1 2.05 2.05l-.04.04a1.2 1.2 0 0 0-.24 1.32v.06a1.2 1.2 0 0 0 1.1.73h.12a1.45 1.45 0 0 1 0 2.9h-.06a1.2 1.2 0 0 0-1.1.73z" />
    </svg>
  ),
};

export default function Toolbar({ runState, onAction }) {
  const isRunning = runState === "running";

  return (
    <div className="html-toolbar">
      <div className="toolbar-group">
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("toggleSidebar")}
          title="Toggle Sidebar"
        >
          {icons.sidebar}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("openFolder")}
          title="Open Folder"
        >
          {icons.folder}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("openFile")}
          title="Open File"
        >
          {icons.file}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("newSnippet")}
          title="New Snippet"
        >
          {icons.newFile}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("format")}
          title="Format"
        >
          {icons.format}
        </button>
        <button
          type="button"
          className={`toolbar-btn${isRunning ? " running" : ""}`}
          onClick={() => onAction("run")}
          title={isRunning ? "Stop" : "Run"}
        >
          {isRunning ? icons.stop : icons.play}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("share")}
          title="Share to Playground"
        >
          {icons.share}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("import")}
          title="Import from Playground"
        >
          {icons.import}
        </button>
      </div>
      <div className="toolbar-spacer" />
      <div className="toolbar-group">
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("rerun")}
          title="Rerun Last"
        >
          {icons.rerun}
        </button>
        <button
          type="button"
          className="toolbar-btn"
          onClick={() => onAction("settings")}
          title="Settings"
        >
          {icons.settings}
        </button>
      </div>
    </div>
  );
}

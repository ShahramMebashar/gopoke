function bridge() {
  return window.go && window.go.desktop && window.go.desktop.WailsBridge
    ? window.go.desktop.WailsBridge
    : null;
}

function runtimeApi() {
  return window.runtime ? window.runtime : null;
}

function requireBridge() {
  const api = bridge();
  if (!api) {
    throw new Error("Wails bridge unavailable. Run from the desktop app.");
  }
  return api;
}

export async function openProject(path) {
  return requireBridge().OpenProject(path);
}

export async function chooseProjectDirectory() {
  return requireBridge().ChooseProjectDirectory();
}

export async function recentProjects(limit = 12) {
  return requireBridge().RecentProjects(limit);
}

export async function setProjectDefaultPackage(projectPath, packagePath) {
  return requireBridge().SetProjectDefaultPackage(projectPath, packagePath);
}

export async function projectEnvVars(projectPath) {
  return requireBridge().ProjectEnvVars(projectPath);
}

export async function upsertProjectEnvVar(projectPath, key, value, masked) {
  return requireBridge().UpsertProjectEnvVar(projectPath, key, value, masked);
}

export async function deleteProjectEnvVar(projectPath, key) {
  return requireBridge().DeleteProjectEnvVar(projectPath, key);
}

export async function setProjectWorkingDirectory(projectPath, workingDirectory) {
  return requireBridge().SetProjectWorkingDirectory(projectPath, workingDirectory);
}

export async function availableToolchains() {
  return requireBridge().AvailableToolchains();
}

export async function setProjectToolchain(projectPath, toolchain) {
  return requireBridge().SetProjectToolchain(projectPath, toolchain);
}

export async function projectSnippets(projectPath) {
  return requireBridge().ProjectSnippets(projectPath);
}

export async function saveProjectSnippet(projectPath, snippetId, name, content) {
  return requireBridge().SaveProjectSnippet(projectPath, snippetId, name, content);
}

export async function deleteProjectSnippet(projectPath, snippetId) {
  return requireBridge().DeleteProjectSnippet(projectPath, snippetId);
}

export async function formatSnippet(source) {
  return requireBridge().FormatSnippet(source);
}

export async function runSnippet(request) {
  return requireBridge().RunSnippet(request);
}

export async function cancelRun(runId) {
  return requireBridge().CancelRun(runId);
}

export function onRunStdoutChunk(callback) {
  const runtime = runtimeApi();
  if (!runtime || typeof runtime.EventsOn !== "function") {
    return () => {};
  }

  const cancel = runtime.EventsOn("gopad:run:stdout-chunk", callback);
  if (typeof cancel === "function") {
    return cancel;
  }
  if (typeof runtime.EventsOff === "function") {
    return () => runtime.EventsOff("gopad:run:stdout-chunk");
  }
  return () => {};
}

export function onRunStderrChunk(callback) {
  const runtime = runtimeApi();
  if (!runtime || typeof runtime.EventsOn !== "function") {
    return () => {};
  }

  const cancel = runtime.EventsOn("gopad:run:stderr-chunk", callback);
  if (typeof cancel === "function") {
    return cancel;
  }
  if (typeof runtime.EventsOff === "function") {
    return () => runtime.EventsOff("gopad:run:stderr-chunk");
  }
  return () => {};
}
